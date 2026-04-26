package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

const (
	documentAITaskSynopsis      = "synopsis"
	documentAITaskExtractFields = "extract_fields"
	documentAITaskAnswer        = "answer_question"

	maxDocumentBytes = int64(5 << 20) // 5MB for AI context extraction
	maxContextChars  = 100000
)

type documentAIRequest struct {
	Task          string   `json:"task"`
	Question      string   `json:"question"`
	Fields        []string `json:"fields"`
	IntegrationID string   `json:"integration_id"`
}

type documentAIResponse struct {
	DocumentID    string                 `json:"document_id"`
	Task          string                 `json:"task"`
	IntegrationID string                 `json:"integration_id"`
	Provider      string                 `json:"provider"`
	Model         string                 `json:"model"`
	Output        string                 `json:"output"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type documentAIIntegrationResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	EndpointURL string `json:"endpoint_url"`
}

var splitTokenRegex = regexp.MustCompile(`[^a-z0-9]+`)

// ListDocumentAIIntegrationsHandler returns active AI integrations that can be used by document workflows.
func ListDocumentAIIntegrationsHandler(w http.ResponseWriter, r *http.Request) {
	var items []models.ThirdPartyIntegration
	if err := config.DB.
		Where("status = ?", models.IntegrationStatusActive).
		Order("name ASC").
		Find(&items).Error; err != nil {
		http.Error(w, "failed to load ai integrations", http.StatusInternalServerError)
		return
	}

	resp := make([]documentAIIntegrationResponse, 0, len(items))
	for _, item := range items {
		if !integrationHasScope(&item, models.IntegrationScopeDocumentAIUse) {
			continue
		}
		if strings.TrimSpace(item.Provider) == "" || strings.TrimSpace(item.Model) == "" || strings.TrimSpace(item.EndpointURL) == "" {
			continue
		}

		resp = append(resp, documentAIIntegrationResponse{
			ID:          item.ID.String(),
			Name:        item.Name,
			Description: item.Description,
			Provider:    item.Provider,
			Model:       item.Model,
			EndpointURL: item.EndpointURL,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"integrations": resp,
		"total":        len(resp),
	})
}

// ProcessDocumentAIHandler performs document-only AI operations.
func ProcessDocumentAIHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getDocumentUserID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	documentID := vars["id"]
	if _, err := uuid.Parse(documentID); err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}

	var req documentAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	req.Task = strings.TrimSpace(strings.ToLower(req.Task))
	req.IntegrationID = strings.TrimSpace(req.IntegrationID)

	if req.Task == "" {
		http.Error(w, "task is required", http.StatusBadRequest)
		return
	}
	if req.IntegrationID == "" {
		http.Error(w, "integration_id is required", http.StatusBadRequest)
		return
	}

	if req.Task != documentAITaskSynopsis && req.Task != documentAITaskExtractFields && req.Task != documentAITaskAnswer {
		http.Error(w, "unsupported task. allowed: synopsis, extract_fields, answer_question", http.StatusBadRequest)
		return
	}

	if req.Task == documentAITaskExtractFields && len(req.Fields) == 0 {
		http.Error(w, "fields are required for extract_fields task", http.StatusBadRequest)
		return
	}

	if req.Task == documentAITaskAnswer {
		req.Question = strings.TrimSpace(req.Question)
		if req.Question == "" {
			http.Error(w, "question is required for answer_question task", http.StatusBadRequest)
			return
		}
	}

	integration, err := resolveDocumentAIIntegration(req.IntegrationID)
	if err != nil {
		http.Error(w, "invalid integration configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	var document models.Document
	if err := config.DB.First(&document, "id = ?", documentID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "document not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch document: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	contextText, extractionMeta, err := extractDocumentTextForAI(&document)
	if err != nil {
		http.Error(w, "failed to read document text for ai: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	if req.Task == documentAITaskAnswer {
		if !isQuestionInDocumentScope(req.Question, contextText) {
			http.Error(w, "query rejected: only document-related questions are allowed", http.StatusUnprocessableEntity)
			return
		}
	}

	prompt := buildDocumentAIPrompt(&document, req, contextText)

	output, err := callDocumentAIProvider(integration, prompt)
	if err != nil {
		http.Error(w, "document ai request failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(output)), "OUT_OF_SCOPE") {
		http.Error(w, "query rejected: only document-related requests are allowed", http.StatusUnprocessableEntity)
		return
	}

	auditLog := models.DocumentAuditLog{
		DocumentID: document.ID,
		UserID:     &userID,
		Action:     models.DocumentAuditActionEdit,
		Details: models.DocumentMetadata{
			"ai_task":        req.Task,
			"integration_id": integration.ID,
			"provider":       integration.Provider,
			"model":          integration.Model,
		},
		IPAddress: r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}
	config.DB.Create(&auditLog)

	resp := documentAIResponse{
		DocumentID:    document.ID.String(),
		Task:          req.Task,
		IntegrationID: integration.ID.String(),
		Provider:      integration.Provider,
		Model:         integration.Model,
		Output:        strings.TrimSpace(output),
		Metadata:      extractionMeta,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func resolveDocumentAIIntegration(integrationID string) (*models.ThirdPartyIntegration, error) {
	id, err := uuid.Parse(integrationID)
	if err != nil {
		return nil, errors.New("integration_id must be a valid UUID")
	}

	var integration models.ThirdPartyIntegration
	if err := config.DB.First(&integration, "id = ? AND status = ?", id, models.IntegrationStatusActive).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("integration is not active or does not exist")
		}
		return nil, err
	}

	if !integrationHasScope(&integration, models.IntegrationScopeDocumentAIUse) {
		return nil, errors.New("integration is missing required scope integration.document.ai.use")
	}

	if strings.TrimSpace(integration.Provider) == "" || strings.TrimSpace(integration.EndpointURL) == "" || strings.TrimSpace(integration.Model) == "" {
		return nil, errors.New("integration must define provider, endpoint_url and model from integration screen")
	}

	if strings.TrimSpace(integration.SecretCipher) == "" {
		return nil, errors.New("integration secret is not configured")
	}

	normalized, err := normalizeIntegrationURL(integration.EndpointURL)
	if err != nil {
		return nil, errors.New("integration endpoint_url is invalid")
	}
	if !integrationAllowsURL(&integration, normalized) {
		return nil, errors.New("integration endpoint_url is not allowlisted")
	}

	if strings.TrimSpace(integration.AuthHeader) == "" {
		integration.AuthHeader = "Authorization"
	}
	if strings.TrimSpace(integration.AuthScheme) == "" {
		integration.AuthScheme = "Bearer"
	}

	integration.Provider = strings.ToLower(strings.TrimSpace(integration.Provider))

	return &integration, nil
}

func integrationHasScope(integration *models.ThirdPartyIntegration, scope string) bool {
	for _, item := range integration.DataScopes {
		if item == scope {
			return true
		}
	}
	return false
}

func integrationAllowsURL(integration *models.ThirdPartyIntegration, normalizedURL string) bool {
	for _, item := range integration.AllowedURLs {
		if strings.TrimSpace(item) == normalizedURL {
			return true
		}
	}
	return false
}

func extractDocumentTextForAI(doc *models.Document) (string, map[string]interface{}, error) {
	reader, _, err := openStoredFileReader(context.Background(), doc.FilePath)
	if err != nil {
		return "", nil, err
	}
	defer reader.Close()

	limited := io.LimitReader(reader, maxDocumentBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", nil, err
	}

	ext := strings.ToLower(strings.TrimPrefix(doc.FileExtension, "."))
	contentType := strings.ToLower(doc.FileType)

	var text string
	switch {
	case isLikelyTextContent(ext, contentType):
		text = sanitizeUTF8(string(data))
	case ext == "docx":
		text, err = extractDOCXText(data)
		if err != nil {
			return "", nil, err
		}
	default:
		return "", nil, fmt.Errorf("unsupported document type for ai extraction: %s", doc.FileExtension)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, errors.New("document has no extractable text")
	}

	if len(text) > maxContextChars {
		text = text[:maxContextChars]
	}

	meta := map[string]interface{}{
		"context_chars":  len(text),
		"file_type":      doc.FileType,
		"file_extension": doc.FileExtension,
	}

	return text, meta, nil
}

func isLikelyTextContent(ext, contentType string) bool {
	if strings.HasPrefix(contentType, "text/") || strings.Contains(contentType, "json") || strings.Contains(contentType, "xml") {
		return true
	}

	return ext == "txt" || ext == "md" || ext == "csv" || ext == "json" || ext == "xml" || ext == "html" || ext == "htm"
}

func sanitizeUTF8(input string) string {
	clean := strings.ReplaceAll(input, "\u0000", " ")
	return strings.TrimSpace(clean)
}

func extractDOCXText(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		decoder := xml.NewDecoder(rc)
		var builder strings.Builder
		for {
			tok, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}

			switch v := tok.(type) {
			case xml.CharData:
				val := strings.TrimSpace(string(v))
				if val != "" {
					if builder.Len() > 0 {
						builder.WriteString(" ")
					}
					builder.WriteString(val)
				}
			}
		}

		return builder.String(), nil
	}

	return "", errors.New("word/document.xml not found in docx")
}

func isQuestionInDocumentScope(question, contextText string) bool {
	qTokens := normalizeTokens(question)
	if len(qTokens) == 0 {
		return false
	}

	ctxTokenSet := make(map[string]struct{})
	for _, t := range normalizeTokens(contextText) {
		ctxTokenSet[t] = struct{}{}
	}

	matchCount := 0
	for _, t := range qTokens {
		if _, ok := ctxTokenSet[t]; ok {
			matchCount++
		}
	}

	return matchCount > 0
}

func normalizeTokens(text string) []string {
	text = strings.ToLower(text)
	parts := splitTokenRegex.Split(text, -1)

	stop := map[string]struct{}{
		"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "to": {}, "of": {}, "in": {}, "on": {}, "for": {}, "with": {},
		"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "this": {}, "that": {}, "what": {}, "when": {}, "where": {}, "who": {}, "why": {}, "how": {},
	}

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if _, ok := stop[p]; ok {
			continue
		}
		tokens = append(tokens, p)
	}

	seen := map[string]struct{}{}
	uniq := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniq = append(uniq, t)
	}
	sort.Strings(uniq)
	return uniq
}

func buildDocumentAIPrompt(doc *models.Document, req documentAIRequest, contextText string) string {
	var taskInstruction string
	switch req.Task {
	case documentAITaskSynopsis:
		taskInstruction = "Create a concise synopsis of this document in 6-10 bullet points and a 1 paragraph summary."
	case documentAITaskExtractFields:
		taskInstruction = fmt.Sprintf("Extract only these fields from the document: %s. Return strict JSON object with these exact keys and null when missing.", strings.Join(req.Fields, ", "))
	case documentAITaskAnswer:
		taskInstruction = fmt.Sprintf("Answer this question using only the document content: %q. If not found in document, return: OUT_OF_SCOPE: question is not answerable from document.", req.Question)
	}

	return fmt.Sprintf(`You are a restricted document assistant.
You must only do document-processing tasks.
If asked anything outside document content or unrelated tasks, return exactly:
OUT_OF_SCOPE: non-document request

Document metadata:
- title: %s
- file_name: %s
- file_type: %s

Task:
%s

Document content:
"""
%s
"""`, doc.Title, doc.FileName, doc.FileType, taskInstruction, contextText)
}

func callDocumentAIProvider(integration *models.ThirdPartyIntegration, prompt string) (string, error) {
	switch integration.Provider {
	case "openai", "chatgpt":
		return callOpenAI(integration, prompt)
	case "claude", "anthropic":
		return callClaude(integration, prompt)
	default:
		return "", fmt.Errorf("unsupported provider: %s", integration.Provider)
	}
}

func callOpenAI(integration *models.ThirdPartyIntegration, prompt string) (string, error) {
	apiKey, err := decryptIntegrationSecret(integration.SecretCipher)
	if err != nil {
		return "", err
	}

	body := map[string]interface{}{
		"model": integration.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, integration.EndpointURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set(integration.AuthHeader, strings.TrimSpace(integration.AuthScheme+" "+apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai api error: %s", string(respBytes))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", errors.New("openai returned no choices")
	}

	return payload.Choices[0].Message.Content, nil
}

func callClaude(integration *models.ThirdPartyIntegration, prompt string) (string, error) {
	apiKey, err := decryptIntegrationSecret(integration.SecretCipher)
	if err != nil {
		return "", err
	}

	body := map[string]interface{}{
		"model":       integration.Model,
		"max_tokens":  1024,
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, integration.EndpointURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set(integration.AuthHeader, strings.TrimSpace(integration.AuthScheme+" "+apiKey))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("claude api error: %s", string(respBytes))
	}

	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &payload); err != nil {
		return "", err
	}
	if len(payload.Content) == 0 {
		return "", errors.New("claude returned no content")
	}

	return payload.Content[0].Text, nil
}
