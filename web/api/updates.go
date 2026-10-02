package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/scopes"
)

var updateRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type updateInfo struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	URL             string `json:"url"`
	Notes           string `json:"notes"`
	PublishedAt     string `json:"publishedAt"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CanApply        bool   `json:"canApply"`
	Running         bool   `json:"running"`
	LastResult      string `json:"lastResult,omitempty"`
}

var updateState struct {
	sync.Mutex
	cached     *updateInfo
	fetchedAt  time.Time
	running    bool
	lastResult string
}

func registerUpdates(g *gin.RouterGroup) {
	g.GET("/update", middleware.RequiresPermission(scopes.ScopeAdmin), getUpdateInfo)
	g.POST("/update/apply", middleware.RequiresPermission(scopes.ScopeAdmin), applyUpdate)
	g.OPTIONS("/update", response.CreateOptions("GET"))
	g.OPTIONS("/update/apply", response.CreateOptions("POST"))
}

func getUpdateInfo(c *gin.Context) {
	refresh := c.Query("refresh") == "true"
	info, err := fetchUpdateInfo(c.Request.Context(), refresh)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not check for updates"})
		return
	}
	c.JSON(http.StatusOK, info)
}

func fetchUpdateInfo(ctx context.Context, force bool) (*updateInfo, error) {
	updateState.Lock()
	defer updateState.Unlock()
	if updateState.cached == nil || force || time.Since(updateState.fetchedAt) > time.Hour {
		repo := config.UpdateRepo.Value()
		if !updateRepoPattern.MatchString(repo) {
			return nil, fmt.Errorf("invalid update repository")
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "PufferPanel/"+pufferpanel.Version)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("release lookup returned HTTP %d", res.StatusCode)
		}
		var release struct {
			Tag         string `json:"tag_name"`
			URL         string `json:"html_url"`
			Body        string `json:"body"`
			PublishedAt string `json:"published_at"`
		}
		if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&release); err != nil {
			return nil, err
		}
		notes := release.Body
		if len(notes) > 4000 {
			notes = notes[:4000]
		}
		// Only link to https URLs so the UI never renders a javascript: link.
		url := release.URL
		if !strings.HasPrefix(url, "https://") {
			url = ""
		}
		updateState.cached = &updateInfo{Latest: release.Tag, URL: url, Notes: notes, PublishedAt: release.PublishedAt}
		updateState.fetchedAt = time.Now()
	}
	info := *updateState.cached
	info.Current = pufferpanel.Version
	info.UpdateAvailable = isNewerVersion(info.Latest, info.Current)
	info.CanApply = config.UpdateCommand.Value() != ""
	info.Running = updateState.running
	info.LastResult = updateState.lastResult
	return &info, nil
}

// applyUpdate runs the operator-configured command; nothing from the request is executed.
func applyUpdate(c *gin.Context) {
	command := config.UpdateCommand.Value()
	if command == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "no update command is configured"})
		return
	}
	updateState.Lock()
	if updateState.running {
		updateState.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "an update is already running"})
		return
	}
	updateState.running = true
	updateState.lastResult = ""
	updateState.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/C", command)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", command)
		}
		out, err := cmd.CombinedOutput()
		result := "ok"
		if err != nil {
			result = "failed: " + err.Error()
		}
		if len(out) > 2000 {
			out = out[len(out)-2000:]
		}
		logging.Info.Printf("panel update finished (%s)\n%s", result, out)
		updateState.Lock()
		updateState.running = false
		updateState.lastResult = result
		updateState.Unlock()
	}()
	c.Status(http.StatusAccepted)
}

// isNewerVersion reports whether latest is newer than current; unparsable versions (e.g. nightly) never are.
func isNewerVersion(latest, current string) bool {
	l, lPre, ok1 := parseVersion(latest)
	cu, cPre, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != cu[i] {
			return l[i] > cu[i]
		}
	}
	// Same core version: a stable release supersedes a running pre-release.
	return !lPre && cPre
}

func parseVersion(v string) (parts [3]int, prerelease bool, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "+"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "-"); i >= 0 {
		v, prerelease = v[:i], true
	}
	fields := strings.Split(v, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return parts, false, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false, false
		}
		parts[i] = n
	}
	return parts, prerelease, true
}
