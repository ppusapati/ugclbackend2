package handlers

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultSubmissionPageSize = 50
	maxSubmissionPageSize     = 200
)

type submissionsCursor struct {
	Timestamp time.Time
	ID        uuid.UUID
}

var (
	submissionsCursorDefaultOnce sync.Once
	submissionsCursorDefaultOn   bool
)

func useSubmissionCursorByDefault() bool {
	submissionsCursorDefaultOnce.Do(func() {
		raw := strings.TrimSpace(strings.ToLower(os.Getenv("SUBMISSIONS_CURSOR_DEFAULT")))
		switch raw {
		case "", "1", "true", "yes", "on":
			submissionsCursorDefaultOn = true
		case "0", "false", "no", "off":
			submissionsCursorDefaultOn = false
		default:
			submissionsCursorDefaultOn = true
		}
	})
	return submissionsCursorDefaultOn
}

func shouldUseSubmissionCursorMode(rawMode string, rawLegacy string, rawCursor string, rawLimit string) bool {
	mode := strings.TrimSpace(strings.ToLower(rawMode))
	legacyFlag := strings.EqualFold(strings.TrimSpace(rawLegacy), "true")

	if legacyFlag || mode == "legacy" || mode == "off" {
		return false
	}

	if mode == "cursor" || mode == "on" {
		return true
	}

	if strings.TrimSpace(rawCursor) != "" || strings.TrimSpace(rawLimit) != "" {
		return true
	}

	return useSubmissionCursorByDefault()
}

func parseSubmissionPageSize(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultSubmissionPageSize, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid limit")
	}

	if value > maxSubmissionPageSize {
		value = maxSubmissionPageSize
	}
	return value, nil
}

func decodeSubmissionsCursor(raw string) (*submissionsCursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor")
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}

	return &submissionsCursor{Timestamp: ts, ID: id}, nil
}

func encodeSubmissionsCursor(ts time.Time, id uuid.UUID) string {
	payload := ts.UTC().Format(time.RFC3339Nano) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}
