package marketplace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"example.com/marketplace-error-handoff/internal/infrai"
)

type writerStub struct {
	err   error
	calls int
}

func (w *writerStub) Complete(context.Context, OrderHandoff) error {
	w.calls++
	return w.err
}

type captureStub struct {
	calls   int
	key     string
	payload infrai.CaptureRequest
}

func (c *captureStub) Capture(_ context.Context, key string, payload infrai.CaptureRequest) error {
	c.calls++
	c.key = key
	c.payload = payload
	return nil
}

func TestOrderHandoffDecision(t *testing.T) {
	base := OrderHandoff{
		EventID: "evt-42",
		OrderID: "ord-42",
		Asset:   SellerAsset{ID: "asset-9", SellerID: "seller-3", State: "ready"},
		Update:  BuyerUpdate{BuyerID: "buyer-8", Confirmed: true},
	}
	backendErr := errors.New("ledger write failed")
	tests := []struct {
		name        string
		change      func(*OrderHandoff)
		writerErr   error
		wantStatus  string
		wantWrite   int
		wantCapture int
	}{
		{name: "ready asset and confirmed buyer", change: func(*OrderHandoff) {}, wantStatus: "handed_off", wantWrite: 1},
		{name: "asset still processing", change: func(h *OrderHandoff) { h.Asset.State = "processing" }, wantStatus: "waiting_for_asset"},
		{name: "buyer has not confirmed", change: func(h *OrderHandoff) { h.Update.Confirmed = false }, wantStatus: "waiting_for_buyer"},
		{name: "backend error is grouped by seller asset", change: func(*OrderHandoff) {}, writerErr: backendErr, wantStatus: "handoff_failed", wantWrite: 1, wantCapture: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handoff := base
			tt.change(&handoff)
			writer := &writerStub{err: tt.writerErr}
			capture := &captureStub{}
			result, _ := (Service{Errors: capture, Writer: writer}).Handoff(context.Background(), handoff)

			if result.Status != tt.wantStatus || writer.calls != tt.wantWrite || capture.calls != tt.wantCapture {
				t.Fatalf("result=%+v writes=%d captures=%d", result, writer.calls, capture.calls)
			}
			if tt.wantCapture == 1 {
				wantFingerprint := []string{"order-handoff", "seller-3", "asset-9"}
				if !reflect.DeepEqual(capture.payload.Fingerprint, wantFingerprint) {
					t.Fatalf("fingerprint = %v, want %v", capture.payload.Fingerprint, wantFingerprint)
				}
				if capture.key != "handoff-evt-42" {
					t.Fatalf("idempotency key = %q", capture.key)
				}
			}
		})
	}
}
