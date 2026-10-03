package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
var updateTagPattern = regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$`)
var updateCommitPattern = regexp.MustCompile(`^commit:([0-9a-fA-F]{7,40})$`)
var updateFullCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var updateHexPattern = regexp.MustCompile(`^[0-9a-f]+$`)

type updateAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type githubRelease struct {
	Tag         string `json:"tag_name"`
	Name        string `json:"name"`
	URL         string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

type githubCommit struct {
	SHA    string `json:"sha"`
	URL    string `json:"html_url"`
	Commit struct {
		Message   string `json:"message"`
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type updateRelease struct {
	Tag         string        `json:"tag"`
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	PublishedAt string        `json:"publishedAt"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []updateAsset `json:"assets"`
}

type updateInfo struct {
	Current         string        `json:"current"`
	CurrentHash     string        `json:"currentHash"`
	Latest          string        `json:"latest"`
	URL             string        `json:"url"`
	Notes           string        `json:"notes"`
	PublishedAt     string        `json:"publishedAt"`
	UpdateAvailable bool          `json:"updateAvailable"`
	CanApply        bool          `json:"canApply"`
	Running         bool          `json:"running"`
	LastResult      string        `json:"lastResult,omitempty"`
	Assets          []updateAsset `json:"assets"`
}

var updateState struct {
	sync.Mutex
	cached       *updateInfo
	cachedTag    string
	cachedRepo   string
	fetchedAt    time.Time
	releases     []updateRelease
	releasesRepo string
	releasesAt   time.Time
	running      bool
	lastResult   string
}

func registerUpdates(g *gin.RouterGroup) {
	g.GET("/system", middleware.RequiresPermission(scopes.ScopeAdmin), getSystemInfo)
	g.OPTIONS("/system", response.CreateOptions("GET"))
	g.GET("/update", middleware.RequiresPermission(scopes.ScopeAdmin), getUpdateInfo)
	g.GET("/update/releases", middleware.RequiresPermission(scopes.ScopeAdmin), getUpdateReleases)
	g.POST("/update/apply", middleware.RequiresPermission(scopes.ScopeAdmin), applyUpdate)
	g.OPTIONS("/update", response.CreateOptions("GET"))
	g.OPTIONS("/update/releases", response.CreateOptions("GET"))
	g.OPTIONS("/update/apply", response.CreateOptions("POST"))
}

var processStart = time.Now()

// getSystemInfo exposes build and runtime facts only; no config values or secrets.
func getSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":       pufferpanel.Version,
		"hash":          pufferpanel.Hash,
		"goVersion":     runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"uptimeSeconds": int64(time.Since(processStart).Seconds()),
		"database":      config.DatabaseDialect.Value(),
	})
}

func getUpdateInfo(c *gin.Context) {
	refresh := c.Query("refresh") == "true"
	tag, err := normalizeUpdateTag(c.Query("version"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	info, err := fetchUpdateInfo(c.Request.Context(), refresh, tag)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not check for updates"})
		return
	}
	c.JSON(http.StatusOK, info)
}

func normalizeUpdateTag(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.EqualFold(tag, "latest") {
		return "", nil
	}
	if updateCommitPattern.MatchString(tag) {
		return "commit:" + strings.ToLower(strings.TrimPrefix(strings.ToLower(tag), "commit:")), nil
	}
	if !updateTagPattern.MatchString(tag) {
		return "", fmt.Errorf("invalid release version")
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	return tag, nil
}

func getUpdateReleases(c *gin.Context) {
	force := c.Query("refresh") == "true"
	repo := config.UpdateRepo.Value()
	updateState.Lock()
	if !force && updateState.releases != nil && updateState.releasesRepo == repo && time.Since(updateState.releasesAt) < time.Hour {
		versions := append([]updateRelease(nil), updateState.releases...)
		updateState.Unlock()
		c.JSON(http.StatusOK, versions)
		return
	}
	updateState.Unlock()

	if !updateRepoPattern.MatchString(repo) {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "invalid update repository"}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases?per_page=30", nil)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "PufferPanel/"+pufferpanel.Version)
	result, err := http.DefaultClient.Do(request)
	if response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	defer func() { _ = result.Body.Close() }()
	if result.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": fmt.Sprintf("release lookup returned HTTP %d", result.StatusCode)}})
		return
	}

	var releases []githubRelease
	if err = json.NewDecoder(io.LimitReader(result.Body, 4<<20)).Decode(&releases); response.HandleError(c, err, http.StatusBadGateway) {
		return
	}
	versions := make([]updateRelease, 0, len(releases))
	for _, release := range releases {
		if _, err = normalizeUpdateTag(release.Tag); err != nil {
			continue
		}
		versions = append(versions, toUpdateRelease(release))
	}
	commits, err := fetchGitHubCommits(c.Request.Context(), repo, "", 1)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not check repository commits"}})
		return
	}
	for _, commit := range commits {
		if !updateFullCommitPattern.MatchString(commit.SHA) {
			continue
		}
		versions = append(versions, toCommitUpdateRelease(commit))
	}
	updateState.Lock()
	updateState.releases = versions
	updateState.releasesRepo = repo
	updateState.releasesAt = time.Now()
	updateState.Unlock()
	c.JSON(http.StatusOK, versions)
}

func fetchUpdateInfo(ctx context.Context, force bool, requestedTag string) (*updateInfo, error) {
	updateState.Lock()
	defer updateState.Unlock()
	repo := config.UpdateRepo.Value()
	cacheTag := requestedTag
	if cacheTag == "" {
		cacheTag = "latest"
	}
	if updateState.cached == nil || updateState.cachedTag != cacheTag || updateState.cachedRepo != repo || force || time.Since(updateState.fetchedAt) > time.Hour {
		if !updateRepoPattern.MatchString(repo) {
			return nil, fmt.Errorf("invalid update repository")
		}
		if requestedTag == "" || updateCommitPattern.MatchString(requestedTag) {
			sha := strings.TrimPrefix(requestedTag, "commit:")
			commits, err := fetchGitHubCommits(ctx, repo, sha, 1)
			if err != nil || len(commits) == 0 {
				if err == nil {
					err = fmt.Errorf("repository has no commits")
				}
				return nil, err
			}
			commit := commits[0]
			if !updateFullCommitPattern.MatchString(commit.SHA) {
				return nil, fmt.Errorf("GitHub returned an invalid commit hash")
			}
			currentHash := strings.ToLower(strings.TrimSpace(pufferpanel.Hash))
			updateState.cached = &updateInfo{
				Latest:      "commit:" + strings.ToLower(commit.SHA),
				URL:         commit.URL,
				Notes:       commit.Commit.Message,
				PublishedAt: commit.Commit.Committer.Date,
			}
			if len(updateState.cached.Notes) > 4000 {
				updateState.cached.Notes = updateState.cached.Notes[:4000]
			}
			updateState.cachedTag = cacheTag
			updateState.cachedRepo = repo
			updateState.fetchedAt = time.Now()
			info := *updateState.cached
			info.Current = pufferpanel.Version
			info.CurrentHash = currentHash
			info.UpdateAvailable = !sameCommit(currentHash, commit.SHA)
			info.CanApply = config.UpdateCommand.Value() != ""
			info.Running = updateState.running
			info.LastResult = updateState.lastResult
			return &info, nil
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		endpoint := "/releases/latest"
		if requestedTag != "" {
			endpoint = "/releases/tags/" + url.PathEscape(requestedTag)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+endpoint, nil)
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
		var release githubRelease
		if err = json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&release); err != nil {
			return nil, err
		}
		if _, err = normalizeUpdateTag(release.Tag); err != nil {
			return nil, err
		}
		notes := release.Body
		if len(notes) > 4000 {
			notes = notes[:4000]
		}
		// Only link to https URLs so the UI never renders a javascript: link.
		option := toUpdateRelease(release)
		updateState.cached = &updateInfo{Latest: release.Tag, URL: option.URL, Notes: notes, PublishedAt: release.PublishedAt, Assets: option.Assets}
		updateState.cachedTag = cacheTag
		updateState.cachedRepo = repo
		updateState.fetchedAt = time.Now()
	}
	info := *updateState.cached
	info.Current = pufferpanel.Version
	info.CurrentHash = pufferpanel.Hash
	info.UpdateAvailable = isNewerVersion(info.Latest, info.Current)
	info.CanApply = config.UpdateCommand.Value() != ""
	info.Running = updateState.running
	info.LastResult = updateState.lastResult
	return &info, nil
}

func toUpdateRelease(release githubRelease) updateRelease {
	option := updateRelease{Tag: release.Tag, Name: release.Name, URL: release.URL, PublishedAt: release.PublishedAt, Prerelease: release.Prerelease}
	if !strings.HasPrefix(option.URL, "https://") {
		option.URL = ""
	}
	for _, asset := range release.Assets {
		if !strings.HasPrefix(asset.URL, "https://") {
			continue
		}
		option.Assets = append(option.Assets, updateAsset{Name: asset.Name, URL: asset.URL, Size: asset.Size})
	}
	return option
}

func fetchGitHubCommits(ctx context.Context, repo, sha string, limit int) ([]githubCommit, error) {
	if !updateRepoPattern.MatchString(repo) {
		return nil, fmt.Errorf("invalid update repository")
	}
	if limit < 1 || limit > 100 {
		limit = 1
	}
	endpoint := "https://api.github.com/repos/" + repo + "/commits"
	if sha != "" {
		endpoint += "/" + url.PathEscape(sha)
	} else {
		endpoint += "?per_page=" + strconv.Itoa(limit)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "PufferPanel/"+pufferpanel.Version)
	result, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Body.Close() }()
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("commit lookup returned HTTP %d", result.StatusCode)
	}
	if sha != "" {
		var commit githubCommit
		if err = json.NewDecoder(io.LimitReader(result.Body, 1<<20)).Decode(&commit); err != nil {
			return nil, err
		}
		return []githubCommit{commit}, nil
	}
	var commits []githubCommit
	if err = json.NewDecoder(io.LimitReader(result.Body, 4<<20)).Decode(&commits); err != nil {
		return nil, err
	}
	return commits, nil
}

func toCommitUpdateRelease(commit githubCommit) updateRelease {
	sha := strings.ToLower(commit.SHA)
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	message := strings.TrimSpace(strings.SplitN(commit.Commit.Message, "\n", 2)[0])
	name := "Commit " + shortSHA
	if message != "" {
		name += " · " + message
	}
	link := commit.URL
	if !strings.HasPrefix(link, "https://github.com/") {
		link = ""
	}
	return updateRelease{
		Tag:         "commit:" + sha,
		Name:        name,
		URL:         link,
		PublishedAt: commit.Commit.Committer.Date,
	}
}

func sameCommit(current, target string) bool {
	current = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(current)), "commit:")
	target = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(target)), "commit:")
	if len(current) < 7 || len(target) < 7 || !updateHexPattern.MatchString(current) || !updateHexPattern.MatchString(target) {
		return false
	}
	if len(current) > len(target) {
		current, target = target, current
	}
	return strings.HasPrefix(target, current)
}

// applyUpdate runs the operator-configured command; nothing from the request is executed.
func applyUpdate(c *gin.Context) {
	command := config.UpdateCommand.Value()
	if command == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "no update command is configured"})
		return
	}
	var request struct {
		Version string `json:"version"`
	}
	if c.Request.Body != nil {
		if err := json.NewDecoder(c.Request.Body).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			response.HandleError(c, err, http.StatusBadRequest)
			return
		}
	}
	tag, err := normalizeUpdateTag(request.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"msg": err.Error()}})
		return
	}
	info, err := fetchUpdateInfo(c.Request.Context(), false, tag)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"msg": "could not validate selected release"}})
		return
	}
	if !info.UpdateAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"msg": "selected release is not newer than the installed version"}})
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
		cmd.Env = append(os.Environ(), "PANEL_UPDATE_VERSION="+info.Latest, "PANEL_UPDATE_REPO="+config.UpdateRepo.Value())
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
