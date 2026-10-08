package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/response"
)

const playitIPCVersion = 2

type playitIPCFrame struct {
	MessageKind string          `json:"message_kind"`
	Data        json.RawMessage `json:"data"`
}

type playitHello struct {
	Protocol struct {
		IPCVersion uint32 `json:"ipc_version"`
	} `json:"protocol"`
}

type playitIPCResponse struct {
	IPCVersion uint32 `json:"ipc_version"`
	RequestID  uint64 `json:"request_id"`
	Response   struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"response"`
}

type playitIPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var ErrPlayitAgentUnavailable = errors.New("Playit agent is not running or installed")

func registerPlayitRoutes(router *gin.RouterGroup) {
	routes := router.Group("/playit", middleware.ValidateJWT)
	routes.GET("/state", getPlayitState)
	routes.GET("/login-url", getPlayitLoginURL)
	routes.PUT("/secret", provisionPlayitSecret)
	routes.POST("/stop", stopPlayitAgent)
	routes.OPTIONS("/state", response.CreateOptions("GET"))
	routes.OPTIONS("/login-url", response.CreateOptions("GET"))
	routes.OPTIONS("/secret", response.CreateOptions("PUT"))
	routes.OPTIONS("/stop", response.CreateOptions("POST"))
}

func getPlayitState(c *gin.Context) {
	result, err := playitIPCRequest(c.Request.Context(), "get_state", nil)
	if response.HandleError(c, err, http.StatusServiceUnavailable) {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func getPlayitLoginURL(c *gin.Context) {
	result, err := playitIPCRequest(c.Request.Context(), "get_account_login_url", nil)
	if response.HandleError(c, err, http.StatusServiceUnavailable) {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func provisionPlayitSecret(c *gin.Context) {
	var request struct {
		Secret string `json:"secret"`
	}
	if err := c.ShouldBindJSON(&request); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	if strings.TrimSpace(request.Secret) == "" || len(request.Secret) > 4096 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": "a valid Playit secret is required"}})
		return
	}
	result, err := playitIPCRequest(c.Request.Context(), "set_secret", map[string]string{"secret": request.Secret})
	if response.HandleError(c, err, http.StatusServiceUnavailable) {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func stopPlayitAgent(c *gin.Context) {
	result, err := playitIPCRequest(c.Request.Context(), "stop", nil)
	if response.HandleError(c, err, http.StatusServiceUnavailable) {
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func playitIPCRequest(parent context.Context, requestType string, fields map[string]string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	path := playitSocketPath()
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrPlayitAgentUnavailable
		}
	}
	connection, err := dialPlayitAgent(ctx, path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file or directory") {
			return nil, ErrPlayitAgentUnavailable
		}
		return nil, fmt.Errorf("could not connect to Playit agent IPC: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewScanner(connection)
	reader.Buffer(make([]byte, 4096), 1<<20)
	if !reader.Scan() {
		if err := reader.Err(); err != nil {
			return nil, fmt.Errorf("reading Playit IPC hello: %w", err)
		}
		return nil, errors.New("Playit agent closed IPC before hello")
	}
	var hello playitIPCFrame
	if err := json.Unmarshal(reader.Bytes(), &hello); err != nil {
		return nil, fmt.Errorf("invalid Playit IPC hello: %w", err)
	}
	if hello.MessageKind != "hello" {
		return nil, errors.New("Playit agent sent an invalid IPC hello")
	}
	var protocol playitHello
	if err := json.Unmarshal(hello.Data, &protocol); err != nil {
		return nil, fmt.Errorf("invalid Playit IPC protocol info: %w", err)
	}
	if protocol.Protocol.IPCVersion != playitIPCVersion {
		return nil, fmt.Errorf("unsupported Playit IPC version %d", protocol.Protocol.IPCVersion)
	}

	request := map[string]interface{}{"type": requestType}
	for key, value := range fields {
		request[key] = value
	}
	envelope := map[string]interface{}{
		"ipc_version": playitIPCVersion,
		"request_id":  1,
		"request":     request,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(connection, string(encoded)+"\n"); err != nil {
		return nil, fmt.Errorf("writing Playit IPC request: %w", err)
	}
	if !reader.Scan() {
		if err := reader.Err(); err != nil {
			return nil, fmt.Errorf("reading Playit IPC response: %w", err)
		}
		return nil, errors.New("Playit agent closed IPC without a response")
	}
	var frame playitIPCFrame
	if err := json.Unmarshal(reader.Bytes(), &frame); err != nil {
		return nil, fmt.Errorf("invalid Playit IPC response: %w", err)
	}
	if frame.MessageKind != "response" {
		return nil, fmt.Errorf("unexpected Playit IPC frame %q", frame.MessageKind)
	}
	var response playitIPCResponse
	if err := json.Unmarshal(frame.Data, &response); err != nil {
		return nil, fmt.Errorf("invalid Playit IPC response payload: %w", err)
	}
	if response.IPCVersion != playitIPCVersion || response.RequestID != 1 {
		return nil, errors.New("Playit IPC response did not match the request")
	}
	if response.Response.Type == "error" {
		var ipcError playitIPCError
		if err := json.Unmarshal(response.Response.Data, &ipcError); err != nil {
			return nil, fmt.Errorf("Playit agent returned an unreadable error: %w", err)
		}
		return nil, fmt.Errorf("Playit agent %s: %s", ipcError.Code, ipcError.Message)
	}
	return response.Response.Data, nil
}

func playitSocketPath() string {
	if path := config.PlayitSocketPath.Value(); path != "" {
		return path
	}
	switch runtime.GOOS {
	case "linux":
		return "/run/playit/playitd.sock"
	case "windows":
		return `\\.\pipe\playitd-system`
	case "darwin":
		if dir, err := os.UserConfigDir(); err == nil {
			return dir + "/playit_gg/playitd.sock"
		}
	}
	return "./playitd.sock"
}
