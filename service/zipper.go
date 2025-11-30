package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipFiles creates a zip archive from a map of file paths to content
func ZipFiles(files map[string][]byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	defer zipWriter.Close()

	for path, content := range files {
		// Normalize path separators
		path = filepath.ToSlash(path)

		// Create file in zip
		writer, err := zipWriter.Create(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry for %s: %w", path, err)
		}

		// Write content
		if _, err := writer.Write(content); err != nil {
			return nil, fmt.Errorf("failed to write content for %s: %w", path, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ReadDirectoryFiles reads all files from a directory recursively
func ReadDirectoryFiles(rootDir string) (map[string][]byte, error) {
	files := make(map[string][]byte)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Get relative path from root
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		files[relPath] = content
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	return files, nil
}

// WriteFilesToDirectory writes files from a map to a directory
func WriteFilesToDirectory(files map[string][]byte, rootDir string) error {
	for path, content := range files {
		fullPath := filepath.Join(rootDir, path)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", path, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
	}

	return nil
}

// ExtractZip extracts a zip archive to a directory
func ExtractZip(zipData []byte, destDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, file := range reader.File {
		// Prevent zip slip vulnerability
		filePath := filepath.Join(destDir, file.Name)
		if !strings.HasPrefix(filePath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(filePath, file.Mode())
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Extract file
		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return fmt.Errorf("failed to open zip file %s: %w", file.Name, err)
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}
