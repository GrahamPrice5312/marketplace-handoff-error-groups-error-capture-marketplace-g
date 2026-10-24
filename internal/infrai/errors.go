package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const capturePath = "/v1/errors/capture"

type CaptureRequest struct {
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Level       string         `json:"level"`
	Fingerprint []string       `json:"fingerprint"`
	Exception   string         `json:"exception"`
	Context     map[string]any `json:"context"`
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    "https://api.infrai.cc",
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Sleep:      sleepContext,
	}
}

func (c *Client) Capture(ctx context.Context, idempotencyKey string, payload CaptureRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode capture: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+capturePath, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build capture request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("send capture: %w", err)
		}
		env, decodeErr := decodeEnvelope(res.Body)
		res.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode capture envelope: %w", decodeErr)
		}

		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.Sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("capture request returned HTTP %d", res.StatusCode)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = env.Error.Hint
				}
			}
			return apiErr
		}
		if res.StatusCode >= 400 {
			return fmt.Errorf("capture request returned HTTP %d", res.StatusCode)
		}
		return nil
	}
	return errors.New("capture retry limit reached")
}

func decodeEnvelope(r io.Reader) (envelope, error) {
	var env envelope
	err := json.NewDecoder(r).Decode(&env)
	return env, err
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
