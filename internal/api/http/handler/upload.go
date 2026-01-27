package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/EternisAI/silo-proxy/internal/api/http/dto"
)

const maxUploadSize = 100 * 1024 * 1024

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

func (h *UploadHandler) HandleUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	targetDir := c.PostForm("target_dir")
	if targetDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_dir is required"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		slog.Error("Failed to read file from form", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .zip files are allowed"})
		return
	}

	tmpFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		slog.Error("Failed to create temp file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process upload"})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, file)
	if err != nil {
		slog.Error("Failed to save uploaded file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	files, err := h.unzipFile(tmpFile.Name(), targetDir)
	if err != nil {
		slog.Error("Failed to unzip file", "error", err, "target_dir", targetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to unzip: %v", err)})
		return
	}

	slog.Info("Successfully extracted zip file", "target_dir", targetDir, "file_count", len(files))
	c.JSON(http.StatusOK, dto.UploadResponse{Files: files})
}

func (h *UploadHandler) HandleDeleteDirectory(c *gin.Context) {
	var req struct {
		TargetDir string `json:"target_dir" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_dir is required"})
		return
	}

	if req.TargetDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_dir cannot be empty"})
		return
	}

	entries, err := os.ReadDir(req.TargetDir)
	if err != nil {
		slog.Error("Failed to read directory", "error", err, "target_dir", req.TargetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read directory: %v", err)})
		return
	}

	for _, entry := range entries {
		entryPath := filepath.Join(req.TargetDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			slog.Error("Failed to delete entry", "error", err, "path", entryPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete %s: %v", entryPath, err)})
			return
		}
	}

	slog.Info("Successfully deleted directory contents", "target_dir", req.TargetDir, "deleted_count", len(entries))
	c.JSON(http.StatusOK, dto.DeleteResponse{Message: fmt.Sprintf("Deleted %d items from directory", len(entries))})
}

func (h *UploadHandler) unzipFile(zipPath, destDir string) ([]string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	var extractedFiles []string

	for _, file := range reader.File {
		filePath := filepath.Join(destDir, file.Name)

		if !filepath.IsLocal(file.Name) {
			return nil, fmt.Errorf("invalid file path in zip: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, file.Mode()); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", filePath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
		}

		srcFile, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file in zip %s: %w", file.Name, err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			srcFile.Close()
			return nil, fmt.Errorf("failed to create destination file %s: %w", filePath, err)
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to extract file %s: %w", filePath, err)
		}

		extractedFiles = append(extractedFiles, filePath)
	}

	return extractedFiles, nil
}
