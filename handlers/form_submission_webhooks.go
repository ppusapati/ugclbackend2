package handlers

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
	"p9e.in/ugcl/utils"
)

const formSubmissionWebhookResourceType = "FormSubmission"

func triggerFormSubmissionWebhook(submission *models.FormSubmission) {
	if submission == nil {
		return
	}

	formData := make(map[string]interface{})
	if len(submission.FormData) > 0 {
		if err := json.Unmarshal(submission.FormData, &formData); err != nil {
			log.Printf("⚠️ Failed to unmarshal submission form data for webhook: %v", err)
			return
		}
	}

	triggerFormSubmissionWebhookPayload(
		submission.BusinessVerticalID,
		submission.ID,
		submission.FormCode,
		formData,
	)
}

func triggerDedicatedFormSubmissionWebhook(record *FormSubmissionRecord) {
	if record == nil {
		return
	}

	triggerFormSubmissionWebhookPayload(
		record.BusinessVerticalID,
		record.ID,
		record.FormCode,
		record.FormData,
	)
}

func triggerFormSubmissionWebhookPayload(
	businessID uuid.UUID,
	submissionID uuid.UUID,
	formCode string,
	formData map[string]interface{},
) {
	if formData == nil {
		formData = map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"form_code": formCode,
		"form_data": formData,
	}

	webhookService := utils.NewWebhookService(config.DB)
	if err := webhookService.TriggerWebhook(
		models.EventFormSubmitted,
		formSubmissionWebhookResourceType,
		submissionID.String(),
		businessID,
		payload,
	); err != nil {
		log.Printf("⚠️ Failed to queue form submission webhook for submission %s: %v", submissionID, err)
	}
}
