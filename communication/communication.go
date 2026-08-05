// Package communication notifies requesters via communication-svc's push API.
package communication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type sendPushNotificationRequest struct {
	AccountID int               `json:"account_id"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data"`
}

// SendPush notifies accountID via communication-svc's push endpoint.
func SendPush(ctx context.Context, accountID int, title, body string, data map[string]string) error {
	payload, err := json.Marshal(sendPushNotificationRequest{
		AccountID: accountID,
		Title:     title,
		Body:      body,
		Data:      data,
	})
	if err != nil {
		return fmt.Errorf("marshal push notification: %w", err)
	}

	url := os.Getenv("COMMUNICATION_SERVICE_URL") + "/push/send"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("communication-svc: push/send: status %d", resp.StatusCode)
	}

	return nil
}
