package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/service"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	generator *service.GeneratorService
	github    *service.GitHubService
}

// NewHandler creates a new handler with dependencies
func NewHandler(generator *service.GeneratorService, github *service.GitHubService) *Handler {
	return &Handler{
		generator: generator,
		github:    github,
	}
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string, details ...string) {
	writeJSON(w, status, ErrorResponse{
		Error:   message,
		Details: details,
	})
}

// Health returns the health status of the service
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "gqlgen-rest-api",
	})
}

// Version returns the gqlgen version
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": graphql.Version,
		"service": "gqlgen-rest-api",
	})
}

// Generate handles code generation requests
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var req service.GenerateRequest

	// Check content type
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON request", err.Error())
			return
		}
	} else if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form
		if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
			writeError(w, http.StatusBadRequest, "Failed to parse form", err.Error())
			return
		}

		// Get schema file
		file, _, err := r.FormFile("schema")
		if err != nil {
			writeError(w, http.StatusBadRequest, "Schema file is required", err.Error())
			return
		}
		defer file.Close()

		// Read schema content
		schemaBytes, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to read schema file", err.Error())
			return
		}

		req.Schema = string(schemaBytes)

		// Parse optional config from form
		if configJSON := r.FormValue("config"); configJSON != "" {
			var cfg service.GenerateConfig
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid config JSON", err.Error())
				return
			}
			req.Config = &cfg
		}
	} else {
		writeError(w, http.StatusBadRequest, "Content-Type must be application/json or multipart/form-data")
		return
	}

	// Validate schema
	if strings.TrimSpace(req.Schema) == "" {
		writeError(w, http.StatusBadRequest, "Schema is required")
		return
	}

	// Generate code
	result, err := h.generator.Generate(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Code generation failed", err.Error())
		return
	}

	// Return file list
	fileList := make([]string, 0, len(result.Files))
	for path := range result.Files {
		fileList = append(fileList, path)
	}

	writeJSON(w, http.StatusOK, SuccessResponse{
		Message: "Code generated successfully",
		Data: map[string]interface{}{
			"files": fileList,
			"count": len(fileList),
		},
	})
}

// GenerateZip handles code generation and returns a zip file
func (h *Handler) GenerateZip(w http.ResponseWriter, r *http.Request) {
	var req service.GenerateRequest

	// Check content type
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON request", err.Error())
			return
		}
	} else if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form
		if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
			writeError(w, http.StatusBadRequest, "Failed to parse form", err.Error())
			return
		}

		// Get schema file
		file, _, err := r.FormFile("schema")
		if err != nil {
			writeError(w, http.StatusBadRequest, "Schema file is required", err.Error())
			return
		}
		defer file.Close()

		// Read schema content
		schemaBytes, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to read schema file", err.Error())
			return
		}

		req.Schema = string(schemaBytes)

		// Parse optional config from form
		if configJSON := r.FormValue("config"); configJSON != "" {
			var cfg service.GenerateConfig
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid config JSON", err.Error())
				return
			}
			req.Config = &cfg
		}
	} else {
		writeError(w, http.StatusBadRequest, "Content-Type must be application/json or multipart/form-data")
		return
	}

	// Validate schema
	if strings.TrimSpace(req.Schema) == "" {
		writeError(w, http.StatusBadRequest, "Schema is required")
		return
	}

	// Generate code
	result, err := h.generator.Generate(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Code generation failed", err.Error())
		return
	}

	// Create zip file
	zipData, err := service.ZipFiles(result.Files)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create zip file", err.Error())
		return
	}

	// Return zip file
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=generated-code.zip")
	w.WriteHeader(http.StatusOK)
	w.Write(zipData)
}

// GitHubSyncRequest represents a GitHub sync request
type GitHubSyncRequest struct {
	Schema string                     `json:"schema"`
	Config *service.GenerateConfig    `json:"config,omitempty"`
	GitHub *service.GitHubSyncRequest `json:"github"`
	Token  string                     `json:"token,omitempty"` // Optional override token
}

// GenerateGitHub handles code generation and syncs to GitHub
func (h *Handler) GenerateGitHub(w http.ResponseWriter, r *http.Request) {
	var req GitHubSyncRequest

	// Parse JSON request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request", err.Error())
		return
	}

	// Validate request
	if strings.TrimSpace(req.Schema) == "" {
		writeError(w, http.StatusBadRequest, "Schema is required")
		return
	}
	if req.GitHub == nil {
		writeError(w, http.StatusBadRequest, "GitHub configuration is required")
		return
	}
	if req.GitHub.Owner == "" || req.GitHub.Repo == "" {
		writeError(w, http.StatusBadRequest, "GitHub owner and repo are required")
		return
	}

	// Generate code
	genReq := service.GenerateRequest{
		Schema: req.Schema,
		Config: req.Config,
	}

	result, err := h.generator.Generate(&genReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Code generation failed", err.Error())
		return
	}

	// Determine which GitHub service to use
	githubService := h.github
	if req.Token != "" {
		// Use custom token
		githubService = service.NewGitHubService(req.Token)
	}

	if githubService == nil {
		writeError(w, http.StatusBadRequest, "GitHub token is required (provide in request or configure GITHUB_TOKEN)")
		return
	}

	// Sync to GitHub
	if err := githubService.SyncToGitHub(r.Context(), req.GitHub, result.Files); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to sync to GitHub", err.Error())
		return
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", req.GitHub.Owner, req.GitHub.Repo)

	writeJSON(w, http.StatusOK, SuccessResponse{
		Message: "Code generated and synced to GitHub successfully",
		Data: map[string]interface{}{
			"repository": repoURL,
			"branch":     req.GitHub.Branch,
			"files":      len(result.Files),
		},
	})
}
