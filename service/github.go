package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// GitHubService handles GitHub repository operations
type GitHubService struct {
	client *github.Client
}

// NewGitHubService creates a new GitHub service with authentication
func NewGitHubService(token string) *GitHubService {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GitHubService{
		client: client,
	}
}

// GitHubSyncRequest represents a request to sync code to GitHub
type GitHubSyncRequest struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	Branch        string `json:"branch,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	CreateRepo    bool   `json:"create_repo,omitempty"`
	Private       bool   `json:"private,omitempty"`
}

// SyncToGitHub pushes generated files to a GitHub repository
func (s *GitHubService) SyncToGitHub(ctx context.Context, req *GitHubSyncRequest, files map[string][]byte) error {
	// Set defaults
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.CommitMessage == "" {
		req.CommitMessage = "Update generated GraphQL code"
	}

	// Check if repository exists
	repo, resp, err := s.client.Repositories.Get(ctx, req.Owner, req.Repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			if req.CreateRepo {
				// Create repository
				repo, err = s.createRepository(ctx, req)
				if err != nil {
					return fmt.Errorf("failed to create repository: %w", err)
				}
			} else {
				return fmt.Errorf("repository %s/%s does not exist and create_repo is false", req.Owner, req.Repo)
			}
		} else {
			return fmt.Errorf("failed to check repository: %w", err)
		}
	}

	// Get the default branch if not specified
	if repo.DefaultBranch != nil && req.Branch == "main" {
		req.Branch = *repo.DefaultBranch
	}

	// Get reference to branch
	ref, _, err := s.client.Git.GetRef(ctx, req.Owner, req.Repo, "refs/heads/"+req.Branch)
	if err != nil {
		// Branch doesn't exist, create it from default branch
		if strings.Contains(err.Error(), "404") {
			ref, err = s.createBranch(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get branch reference: %w", err)
		}
	}

	// Get the commit that the branch points to
	commit, _, err := s.client.Git.GetCommit(ctx, req.Owner, req.Repo, *ref.Object.SHA)
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	// Create tree entries for all files
	var entries []*github.TreeEntry
	for path, content := range files {
		// Normalize path
		path = filepath.ToSlash(path)

		// Create blob for file content
		blob := &github.Blob{
			Content:  github.String(string(content)),
			Encoding: github.String("utf-8"),
		}

		createdBlob, _, err := s.client.Git.CreateBlob(ctx, req.Owner, req.Repo, blob)
		if err != nil {
			return fmt.Errorf("failed to create blob for %s: %w", path, err)
		}

		entry := &github.TreeEntry{
			Path: github.String(path),
			Mode: github.String("100644"), // regular file
			Type: github.String("blob"),
			SHA:  createdBlob.SHA,
		}
		entries = append(entries, entry)
	}

	// Create tree
	tree, _, err := s.client.Git.CreateTree(ctx, req.Owner, req.Repo, *commit.Tree.SHA, entries)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	// Create commit
	newCommit := &github.Commit{
		Message: github.String(req.CommitMessage),
		Tree:    tree,
		Parents: []*github.Commit{commit},
	}

	createdCommit, _, err := s.client.Git.CreateCommit(ctx, req.Owner, req.Repo, newCommit, nil)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Update reference
	ref.Object.SHA = createdCommit.SHA
	_, _, err = s.client.Git.UpdateRef(ctx, req.Owner, req.Repo, ref, false)
	if err != nil {
		return fmt.Errorf("failed to update reference: %w", err)
	}

	return nil
}

// createRepository creates a new GitHub repository
func (s *GitHubService) createRepository(ctx context.Context, req *GitHubSyncRequest) (*github.Repository, error) {
	repo := &github.Repository{
		Name:     github.String(req.Repo),
		Private:  github.Bool(req.Private),
		AutoInit: github.Bool(true), // Initialize with README to create default branch
	}

	createdRepo, _, err := s.client.Repositories.Create(ctx, "", repo)
	if err != nil {
		return nil, err
	}

	return createdRepo, nil
}

// createBranch creates a new branch from the default branch
func (s *GitHubService) createBranch(ctx context.Context, req *GitHubSyncRequest) (*github.Reference, error) {
	// Get default branch reference
	defaultRef, _, err := s.client.Git.GetRef(ctx, req.Owner, req.Repo, "refs/heads/main")
	if err != nil {
		// Try master if main doesn't exist
		defaultRef, _, err = s.client.Git.GetRef(ctx, req.Owner, req.Repo, "refs/heads/master")
		if err != nil {
			return nil, fmt.Errorf("failed to get default branch: %w", err)
		}
	}

	// Create new reference
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + req.Branch),
		Object: &github.GitObject{
			SHA: defaultRef.Object.SHA,
		},
	}

	ref, _, err := s.client.Git.CreateRef(ctx, req.Owner, req.Repo, newRef)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
