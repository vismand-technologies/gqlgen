package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// Fetcher fetches Go source files from GitHub repositories
type Fetcher struct {
	client    *http.Client
	token     string
	baseURL   string
	rateLimit *RateLimit
}

// RateLimit tracks GitHub API rate limiting
type RateLimit struct {
	Remaining int
	Reset     time.Time
}

// RepoInfo contains parsed GitHub repository information
type RepoInfo struct {
	Owner string
	Repo  string
	Ref   string // branch, tag, or commit
	Path  string // path within repo
}

// FileContent represents a fetched file
type FileContent struct {
	Path    string
	Content []byte
}

// NewFetcher creates a new GitHub fetcher
func NewFetcher(token string) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:   token,
		baseURL: "https://api.github.com",
	}
}

// ParseImportPath parses a Go import path into GitHub repo info
// e.g., "github.com/user/repo/pkg/models" -> RepoInfo{Owner: "user", Repo: "repo", Path: "pkg/models"}
func ParseImportPath(importPath string) (*RepoInfo, error) {
	if !strings.HasPrefix(importPath, "github.com/") {
		return nil, fmt.Errorf("not a GitHub import path: %s", importPath)
	}

	parts := strings.Split(strings.TrimPrefix(importPath, "github.com/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid GitHub import path: %s", importPath)
	}

	info := &RepoInfo{
		Owner: parts[0],
		Repo:  parts[1],
		Ref:   "", // will be resolved to default branch
	}

	if len(parts) > 2 {
		info.Path = strings.Join(parts[2:], "/")
	}

	return info, nil
}

// FetchPackage fetches all Go files from a package path
func (f *Fetcher) FetchPackage(ctx context.Context, importPath string, ref string) ([]FileContent, error) {
	info, err := ParseImportPath(importPath)
	if err != nil {
		return nil, err
	}

	if ref != "" {
		info.Ref = ref
	} else if info.Ref == "" {
		// Fetch default branch from repo
		defaultBranch, err := f.getDefaultBranch(ctx, info.Owner, info.Repo)
		if err != nil {
			// Fall back to common defaults
			info.Ref = "main"
		} else {
			info.Ref = defaultBranch
		}
	}

	return f.fetchDirectory(ctx, info)
}

// getDefaultBranch fetches the default branch for a repository
func (f *Fetcher) getDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", f.baseURL, owner, repo)

	resp, err := f.doRequest(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repo info: %d", resp.StatusCode)
	}

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "", err
	}

	return repoInfo.DefaultBranch, nil
}

// fetchDirectory fetches all Go files from a directory in the repo
func (f *Fetcher) fetchDirectory(ctx context.Context, info *RepoInfo) ([]FileContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		f.baseURL, info.Owner, info.Repo, info.Path, info.Ref)

	resp, err := f.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package not found: %s/%s/%s", info.Owner, info.Repo, info.Path)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var contents []githubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		// Might be a single file, not a directory
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var files []FileContent
	for _, c := range contents {
		if c.Type != "file" {
			continue
		}
		// Only fetch .go files, skip test files
		if !strings.HasSuffix(c.Name, ".go") || strings.HasSuffix(c.Name, "_test.go") {
			continue
		}

		content, err := f.fetchFile(ctx, info, c.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %w", c.Path, err)
		}

		files = append(files, FileContent{
			Path:    c.Path,
			Content: content,
		})
	}

	return files, nil
}

// fetchFile fetches a single file's content
func (f *Fetcher) fetchFile(ctx context.Context, info *RepoInfo, filePath string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		f.baseURL, info.Owner, info.Repo, filePath, info.Ref)

	resp, err := f.doRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch file: %d", resp.StatusCode)
	}

	var content githubContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, err
	}

	if content.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", content.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content: %w", err)
	}

	return decoded, nil
}

// FetchGoMod fetches the go.mod file to determine the module name
func (f *Fetcher) FetchGoMod(ctx context.Context, owner, repo, ref string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/go.mod?ref=%s",
		f.baseURL, owner, repo, ref)

	resp, err := f.doRequest(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("go.mod not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch go.mod: %d", resp.StatusCode)
	}

	var content githubContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return "", err
	}

	// Parse module name from go.mod
	lines := strings.Split(string(decoded), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module "), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

func (f *Fetcher) doRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "gqlgen-api")

	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}

	// Update rate limit info
	f.updateRateLimit(resp)

	return resp, nil
}

func (f *Fetcher) updateRateLimit(resp *http.Response) {
	if f.rateLimit == nil {
		f.rateLimit = &RateLimit{}
	}

	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		fmt.Sscanf(remaining, "%d", &f.rateLimit.Remaining)
	}

	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		var ts int64
		fmt.Sscanf(reset, "%d", &ts)
		f.rateLimit.Reset = time.Unix(ts, 0)
	}
}

// GetRateLimit returns current rate limit info
func (f *Fetcher) GetRateLimit() *RateLimit {
	return f.rateLimit
}

// PackageName extracts the package name from an import path
func PackageName(importPath string) string {
	return path.Base(importPath)
}

// githubContent represents GitHub API content response
type githubContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"` // "file" or "dir"
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Size     int    `json:"size"`
}
