package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"p9e.in/ugcl/middleware"
)

var errStoredFileNotFound = errors.New("stored file not found")

type managedReadCloser struct {
	io.ReadCloser
	closeFn func() error
}

func (m *managedReadCloser) Close() error {
	err := m.ReadCloser.Close()
	if m.closeFn != nil {
		closeErr := m.closeFn()
		if err == nil {
			return closeErr
		}
	}
	return err
}

func getDocumentUserID(r *http.Request) (uuid.UUID, error) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		return uuid.Nil, errors.New("unauthorized")
	}

	user := middleware.GetUser(r)
	if user.ID == uuid.Nil {
		return uuid.Nil, errors.New("user not found")
	}

	return user.ID, nil
}

func normalizeStoredObjectPath(storagePath string) string {
	trimmed := strings.TrimSpace(storagePath)
	if trimmed == "" {
		return ""
	}

	const gcsPrefix = "https://storage.googleapis.com/"
	if strings.HasPrefix(trimmed, gcsPrefix) {
		remainder := strings.TrimPrefix(trimmed, gcsPrefix)
		parts := strings.SplitN(remainder, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return strings.TrimPrefix(parsed.Path, "/")
	}

	return strings.TrimPrefix(strings.TrimPrefix(trimmed, "./"), "/")
}

func openStoredFileReader(ctx context.Context, storagePath string) (io.ReadCloser, int64, error) {
	if storagePath == "" {
		return nil, 0, errStoredFileNotFound
	}

	if info, err := os.Stat(storagePath); err == nil && !info.IsDir() {
		file, openErr := os.Open(storagePath)
		if openErr != nil {
			return nil, 0, openErr
		}
		return file, info.Size(), nil
	}

	if !useGCSStorage() {
		return nil, 0, errStoredFileNotFound
	}

	if err := validateExpectedGCPProject(); err != nil {
		return nil, 0, err
	}

	client, err := getSharedGCSClient()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get GCS client: %w", err)
	}

	objectName := normalizeStoredObjectPath(storagePath)
	reader, err := client.Bucket(getUploadBucketName()).Object(objectName).NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, 0, errStoredFileNotFound
		}
		return nil, 0, fmt.Errorf("failed to open stored object: %w", err)
	}

	return reader, reader.Attrs.Size, nil
}

func serveStoredFile(w http.ResponseWriter, r *http.Request, storagePath, fileName, fileType string, fileSize int64) error {
	reader, actualSize, err := openStoredFileReader(r.Context(), storagePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if fileSize <= 0 {
		fileSize = actualSize
	}

	if fileName != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	}
	if fileType != "" {
		w.Header().Set("Content-Type", fileType)
	}
	if fileSize > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
	}

	_, err = io.Copy(w, reader)
	return err
}
