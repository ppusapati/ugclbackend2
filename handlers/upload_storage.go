package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const (
	bucketName = "sreeugcl"
)

type storedUpload struct {
	OriginalFilename string
	Filename         string
	URL              string
	Path             string
	Size             int64
	MimeType         string
}

func useGCSStorage() bool {
	return os.Getenv("USE_GCS") == "true" ||
		os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" ||
		os.Getenv("K_SERVICE") != ""
}

func getUploadBucketName() string {
	if bucket := strings.TrimSpace(os.Getenv("UPLOAD_BUCKET_NAME")); bucket != "" {
		return bucket
	}
	return bucketName
}

func resolveActiveGCPProject() string {
	candidates := []string{
		"GOOGLE_CLOUD_PROJECT",
		"GCP_PROJECT",
		"GCLOUD_PROJECT",
		"GCP_PROJECT_ID",
	}

	for _, key := range candidates {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	return ""
}

func validateExpectedGCPProject() error {
	expected := strings.TrimSpace(os.Getenv("EXPECTED_GCP_PROJECT"))
	if expected == "" {
		return nil
	}

	active := resolveActiveGCPProject()
	if active == "" {
		return fmt.Errorf("EXPECTED_GCP_PROJECT is set but active GCP project is not available in env")
	}

	if active != expected {
		return fmt.Errorf("GCP project mismatch: expected %s, got %s", expected, active)
	}

	return nil
}

func storeUploadedFile(r *http.Request, fieldName, localDir string) (*storedUpload, error) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		return nil, fmt.Errorf("bad multipart form: %w", err)
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, fmt.Errorf("missing %s field: %w", fieldName, err)
	}
	defer file.Close()

	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(header.Filename)
	storedName := fmt.Sprintf("%s-%s%s", timestamp, uuid.New().String()[:8], ext)
	mimeType := header.Header.Get("Content-Type")

	if useGCSStorage() {
		if err := validateExpectedGCPProject(); err != nil {
			return nil, err
		}

		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCS client: %w", err)
		}
		defer client.Close()

		uploadBucket := getUploadBucketName()

		prefix := strings.TrimPrefix(filepath.ToSlash(localDir), "./")
		objectName := filepath.ToSlash(filepath.Join(prefix, storedName))

		writer := client.Bucket(uploadBucket).Object(objectName).NewWriter(ctx)
		writer.ContentType = mimeType
		written, err := io.Copy(writer, file)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("failed to upload to GCS: %w", err)
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("failed to finalize GCS upload: %w", err)
		}

		return &storedUpload{
			OriginalFilename: header.Filename,
			Filename:         storedName,
			URL:              fmt.Sprintf("https://storage.googleapis.com/%s/%s", uploadBucket, objectName),
			Path:             objectName,
			Size:             written,
			MimeType:         mimeType,
		}, nil
	}

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	fullPath := filepath.Join(localDir, storedName)
	dst, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	publicPath := "/" + strings.TrimPrefix(filepath.ToSlash(fullPath), "./")

	return &storedUpload{
		OriginalFilename: header.Filename,
		Filename:         storedName,
		URL:              publicPath,
		Path:             fullPath,
		Size:             written,
		MimeType:         mimeType,
	}, nil
}
