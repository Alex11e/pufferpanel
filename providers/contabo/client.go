package contabo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	uuid "github.com/gofrs/uuid/v5"
)

const (
	defaultAPIURL  = "https://api.contabo.com"
	defaultAuthURL = "https://auth.contabo.com/auth/realms/contabo/protocol/openid-connect/token"
)

type Config struct {
	ClientID     string
	ClientSecret string
	APIUser      string
	APIPassword  string
	APIURL       string
	AuthURL      string
}

type Client struct {
	config     Config
	httpClient *http.Client
	tokenMu    sync.Mutex
	token      string
	tokenUntil time.Time
}

func NewClientFromEnv() (*Client, error) {
	config := Config{
		ClientID:     os.Getenv("PUFFER_CONTABO_CLIENT_ID"),
		ClientSecret: os.Getenv("PUFFER_CONTABO_CLIENT_SECRET"),
		APIUser:      os.Getenv("PUFFER_CONTABO_API_USER"),
		APIPassword:  os.Getenv("PUFFER_CONTABO_API_PASSWORD"),
		APIURL:       os.Getenv("PUFFER_CONTABO_API_URL"),
	}
	if config.ClientID == "" || config.ClientSecret == "" || config.APIUser == "" || config.APIPassword == "" {
		return nil, errors.New("Contabo API credentials are not configured")
	}
	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}
	config.AuthURL = defaultAuthURL
	return NewClient(config, &http.Client{Timeout: 30 * time.Second}), nil
}

func NewClient(config Config, httpClient *http.Client) *Client {
	if config.APIURL == "" {
		config.APIURL = defaultAPIURL
	}
	if config.AuthURL == "" {
		config.AuthURL = defaultAuthURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{config: config, httpClient: httpClient}
}

func (c *Client) Do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode Contabo request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	endpoint, err := url.JoinPath(c.config.APIURL, path)
	if err != nil {
		return nil, fmt.Errorf("build Contabo API URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create Contabo API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("x-request-id", uuid.Must(uuid.NewV4()).String())
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Contabo API: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Contabo API response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if len(message) > 512 {
			message = message[:512]
		}
		return nil, fmt.Errorf("Contabo API returned HTTP %d: %s", response.StatusCode, message)
	}
	if len(responseBody) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(responseBody) {
		return nil, errors.New("Contabo API returned invalid JSON")
	}
	return json.RawMessage(responseBody), nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenUntil) {
		return c.token, nil
	}

	form := url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"username":      {c.config.APIUser},
		"password":      {c.config.APIPassword},
		"grant_type":    {"password"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.AuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Contabo authentication request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("authenticate with Contabo: %w", err)
	}
	defer response.Body.Close()

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("Contabo authentication returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("decode Contabo authentication response: %w", err)
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("Contabo authentication response did not include an access token")
	}
	c.token = tokenResponse.AccessToken
	ttl := time.Duration(tokenResponse.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	} else if ttl > time.Minute {
		ttl -= time.Minute
	}
	c.tokenUntil = time.Now().Add(ttl)
	return c.token, nil
}
