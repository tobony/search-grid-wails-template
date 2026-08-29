package pbi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is a tiny JSON-over-HTTP helper for the Power BI API.
type httpClient struct{}

func (h *httpClient) postJSON(ctx context.Context, url, bearer string, body interface{}) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("powerbi request failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAPIError(resp, string(data))
	}
	return data, nil
}
// deleteJSON issues an authenticated DELETE and treats 2xx as success.
func (h *httpClient) deleteJSON(ctx context.Context, url, bearer string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("powerbi request failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp, string(data))
	}
	return nil
}

// apiError is an error that carries the HTTP status from the Power BI
// API so callers can distinguish 401 (auth) and 429 (throttling).
type apiError struct {
	status    int
	retryAfter time.Duration
	msg       string
}

func (e *apiError) Error() string { return e.msg }

// statusOf extracts the HTTP status from an error, or 0.
func statusOf(err error) int {
	if ae, ok := err.(*apiError); ok {
		return ae.status
	}
	return 0
}

// retryAfterOf extracts the Retry-After duration from an error, or 0.
func retryAfterOf(err error) time.Duration {
	if ae, ok := err.(*apiError); ok {
		return ae.retryAfter
	}
	return 0
}

// StatusCode returns the HTTP status carried by err (0 if err is not
// a Power BI API error). Callers outside the package should use this
// instead of matching status numbers in the error text.
func StatusCode(err error) int {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.status
	}
	return 0
}
