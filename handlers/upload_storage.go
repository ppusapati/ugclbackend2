package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const (
	bucketName = "sreeugcl"
)

// sharedGCSClient is initialised at most once and reused across requests.
// Reuse lets the underlying HTTP/2 transport pool TCP connections to GCS,
// eliminating the per-request TLS handshake overhead.
var (
	gcsClientOnce sync.Once
	sharedGCS     *storage.Client
	sharedGCSErr  error
)

// gcsUploadTimeout is the per-operation context deadline for GCS interactions.
// Override with GCS_UPLOAD_TIMEOUT_SECONDS (default 120 s).
func gcsUploadTimeout() time.Duration {
	if s := strings.TrimSpace(os.Getenv("GCS_UPLOAD_TIMEOUT_SECONDS")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return time.Duration(v) * time.Second
		}
		log.Printf("[GCS] invalid GCS_UPLOAD_TIMEOUT_SECONDS %q, using default 120s", s)
	}
	return 120 * time.Second
}

// getSharedGCSClient returns the process-wide GCS client, initialising it on
// the first call.  A background context is used for client creation so that an
// individual request cancellation cannot tear down the shared connection pool.
func getSharedGCSClient() (*storage.Client, error) {
	gcsClientOnce.Do(func() {
		sharedGCS, sharedGCSErr = storage.NewClient(context.Background())
		if sharedGCSErr != nil {
			log.Printf("[GCS] failed to create shared client: %v", sharedGCSErr)
			// Reset Once so a future call can retry after transient failures.
			gcsClientOnce = sync.Once{}
		}
	})
	return sharedGCS, sharedGCSErr
}

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

		client, err := getSharedGCSClient()
		if err != nil {
			return nil, fmt.Errorf("failed to get GCS client: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), gcsUploadTimeout())
		defer cancel()

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
