package config

import (
	"os"
	"strconv"
)

// ServerConfig holds the configuration for the REST server
type ServerConfig struct {
	Port             string
	Host             string
	MaxUploadSize    int64 // in bytes
	TempDir          string
	AllowedOrigins   []string
	GitHubToken      string
	EnableGitHubSync bool
}

// LoadServerConfig loads server configuration from environment variables
func LoadServerConfig() *ServerConfig {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	maxUploadSize := int64(50 * 1024 * 1024) // 50MB default
	if envSize := os.Getenv("MAX_UPLOAD_SIZE"); envSize != "" {
		if size, err := strconv.ParseInt(envSize, 10, 64); err == nil {
			maxUploadSize = size
		}
	}

	tempDir := os.Getenv("TEMP_DIR")
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	allowedOrigins := []string{"*"}
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		allowedOrigins = []string{origins}
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	enableGitHubSync := githubToken != ""

	return &ServerConfig{
		Port:             port,
		Host:             host,
		MaxUploadSize:    maxUploadSize,
		TempDir:          tempDir,
		AllowedOrigins:   allowedOrigins,
		GitHubToken:      githubToken,
		EnableGitHubSync: enableGitHubSync,
	}
}
