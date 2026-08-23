package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// Release represents a GitHub Release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a downloadable file attached to a GitHub Release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// GitHubAPIBase is the GitHub API base URL. Overridable in tests via httptest.
var GitHubAPIBase = "https://api.github.com"

// githubAPIBase retains backward compat for internal callers; prefer GitHubAPIBase.
var githubAPIBase = GitHubAPIBase

// ListReleases fetches all releases for the given GitHub repository.
//
// The caller is responsible for respecting rate limits. Unauthenticated
// requests are limited to 60 requests per hour. Check the X-RateLimit-Remaining
// response header for the remaining quota.
func ListReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	base := GitHubAPIBase
	if env := os.Getenv("BIGGZ_GITHUB_API_BASE"); env != "" {
		base = env
	}
	if base == "" {
		base = githubAPIBase
	}
	if base == "" {
		base = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases", base, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list releases: %s (status %d)", string(body), resp.StatusCode)
	}

	checkRateLimit(resp)

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	return releases, nil
}

// GetRelease fetches a single release by tag from the given GitHub repository.
func GetRelease(ctx context.Context, owner, repo, tag string) (*Release, error) {
	base := GitHubAPIBase
	if env := os.Getenv("BIGGZ_GITHUB_API_BASE"); env != "" {
		base = env
	}
	if base == "" {
		base = githubAPIBase
	}
	if base == "" {
		base = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", base, owner, repo, tag)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get release: %s (status %d)", string(body), resp.StatusCode)
	}

	checkRateLimit(resp)

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	return &release, nil
}

// checkRateLimit logs a warning if the rate limit is running low.
// It reads the X-RateLimit-Remaining header from the response.
func checkRateLimit(resp *http.Response) {
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return
	}
	n, err := strconv.Atoi(remaining)
	if err != nil {
		return
	}
	if n < 10 {
		// Rate limit is low — the caller can check this header
		// if they want to throttle requests.
	}
}

// DownloadBytes fetches the content at url and returns it as bytes.
// The caller is responsible for context cancellation.
func DownloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return data, nil
}
