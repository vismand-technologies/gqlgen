package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/service"
)

func TestHandler_Health(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "healthy") {
		t.Errorf("expected healthy status in response")
	}
}

func TestHandler_Version(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()

	h.Version(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "version") {
		t.Errorf("expected version in response")
	}
}

func TestHandler_Generate_JSON(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	tests := []struct {
		name           string
		request        service.GenerateRequest
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name: "valid schema",
			request: service.GenerateRequest{
				Schema: `type Query { hello: String! }`,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				if !strings.Contains(string(body), "Code generated successfully") {
					t.Errorf("expected success message in response")
				}
			},
		},
		{
			name: "empty schema",
			request: service.GenerateRequest{
				Schema: "",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				if !strings.Contains(string(body), "Schema is required") {
					t.Errorf("expected schema required error")
				}
			},
		},
		{
			name: "whitespace only schema",
			request: service.GenerateRequest{
				Schema: "   \n\t  ",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body []byte) {
				if !strings.Contains(string(body), "Schema is required") {
					t.Errorf("expected schema required error")
				}
			},
		},
		{
			name: "schema with config",
			request: service.GenerateRequest{
				Schema: `type Query { hello: String! }`,
				Config: &service.GenerateConfig{
					ModuleName:   "example.com/test",
					PackageName:  "graph",
					ModelPackage: "model",
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				if !strings.Contains(string(body), "Code generated successfully") {
					t.Errorf("expected success message in response")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.Generate(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if tt.checkResponse != nil {
				tt.checkResponse(t, body)
			}
		})
	}
}

func TestHandler_Generate_InvalidContentType(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader("test"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	h.Generate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Content-Type must be") {
		t.Errorf("expected content type error")
	}
}

func TestHandler_Generate_InvalidJSON(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader("{invalid json}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Generate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_GenerateZip_JSON(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	request := service.GenerateRequest{
		Schema: `type Query { hello: String! }`,
	}
	reqBody, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/api/generate/zip", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.GenerateZip(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %s", contentType)
	}

	contentDisposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") {
		t.Errorf("expected attachment in Content-Disposition")
	}
}

func TestHandler_Generate_Multipart(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add schema file
	part, err := writer.CreateFormFile("schema", "schema.graphql")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte(`type Query { hello: String! }`))

	// Add config
	writer.WriteField("config", `{"module_name": "example.com/test"}`)

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/generate", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Generate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestHandler_Generate_Multipart_NoSchema(t *testing.T) {
	gen := service.NewGeneratorService("")
	h := NewHandler(gen, nil)

	// Create multipart form without schema
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("config", `{"module_name": "example.com/test"}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/generate", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Generate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Schema file is required") {
		t.Errorf("expected schema required error")
	}
}

func TestSuccessResponse(t *testing.T) {
	resp := SuccessResponse{
		Message: "test message",
		Data:    map[string]string{"key": "value"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "test message") {
		t.Error("expected message in JSON")
	}
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Error:   "test error",
		Details: []string{"detail1", "detail2"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "test error") {
		t.Error("expected error in JSON")
	}
	if !strings.Contains(string(data), "detail1") {
		t.Error("expected details in JSON")
	}
}
