package infrai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCaptureRequestBoundary(t *testing.T) {
	tests := []struct {
		name       string
		responses  []int
		wantCalls  int
		wantAPIErr bool
	}{
		{name: "success", responses: []int{http.StatusOK}, wantCalls: 1},
		{name: "business rejection decoded from 400", responses: []int{http.StatusBadRequest}, wantCalls: 1, wantAPIErr: true},
		{name: "rate limit retries", responses: []int{http.StatusTooManyRequests, http.StatusOK}, wantCalls: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost {
					t.Fatalf("method = %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatal("missing bearer key")
				}
				if r.Header.Get("Idempotency-Key") != "handoff-evt-7" {
					t.Fatal("missing idempotency key")
				}
				status := tt.responses[calls-1]
				w.WriteHeader(status)
				if status == http.StatusOK {
					_, _ = w.Write([]byte(`{"ok":true,"data":{"event_id":"evt-7"},"error":null,"metadata":{}}`))
				} else {
					_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"message":"capture request was rejected"},"metadata":{}}`))
				}
			}))
			defer server.Close()

			client := NewClient("test-key")
			client.BaseURL = server.URL
			client.Sleep = func(context.Context, time.Duration) error { return nil }
			err := client.Capture(context.Background(), "handoff-evt-7", CaptureRequest{Title: "handoff failed"})

			var apiErr *APIError
			if errors.As(err, &apiErr) != tt.wantAPIErr {
				t.Fatalf("APIError = %v, error = %v", errors.As(err, &apiErr), err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}
