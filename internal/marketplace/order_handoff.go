package marketplace

import (
	"context"
	"errors"
	"fmt"

	"example.com/marketplace-error-handoff/internal/infrai"
)

var (
	ErrAssetNotReady = errors.New("seller asset is not ready")
	ErrBuyerPending  = errors.New("buyer update is not confirmed")
)

type SellerAsset struct {
	ID       string `json:"id"`
	SellerID string `json:"seller_id"`
	State    string `json:"state"`
}

type BuyerUpdate struct {
	BuyerID   string `json:"buyer_id"`
	Confirmed bool   `json:"confirmed"`
}

type OrderHandoff struct {
	EventID string      `json:"event_id"`
	OrderID string      `json:"order_id"`
	Asset   SellerAsset `json:"asset"`
	Update  BuyerUpdate `json:"buyer_update"`
}

type CaptureClient interface {
	Capture(context.Context, string, infrai.CaptureRequest) error
}

type HandoffWriter interface {
	Complete(context.Context, OrderHandoff) error
}

type Service struct {
	Errors CaptureClient
	Writer HandoffWriter
}

type Result struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (s Service) Handoff(ctx context.Context, handoff OrderHandoff) (Result, error) {
	if handoff.Asset.State != "ready" {
		return Result{OrderID: handoff.OrderID, Status: "waiting_for_asset"}, ErrAssetNotReady
	}
	if !handoff.Update.Confirmed {
		return Result{OrderID: handoff.OrderID, Status: "waiting_for_buyer"}, ErrBuyerPending
	}

	if err := s.Writer.Complete(ctx, handoff); err != nil {
		capture := infrai.CaptureRequest{
			Title:       "marketplace order handoff failed",
			Message:     err.Error(),
			Level:       "error",
			Fingerprint: []string{"order-handoff", handoff.Asset.SellerID, handoff.Asset.ID},
			Exception:   fmt.Sprintf("%T: %v", err, err),
			Context: map[string]any{
				"event_id":  handoff.EventID,
				"order_id":  handoff.OrderID,
				"seller_id": handoff.Asset.SellerID,
				"asset_id":  handoff.Asset.ID,
				"buyer_id":  handoff.Update.BuyerID,
				"stage":     "order_handoff",
			},
		}
		if captureErr := s.Errors.Capture(ctx, "handoff-"+handoff.EventID, capture); captureErr != nil {
			return Result{OrderID: handoff.OrderID, Status: "capture_rejected"}, captureErr
		}
		return Result{OrderID: handoff.OrderID, Status: "handoff_failed"}, err
	}

	return Result{OrderID: handoff.OrderID, Status: "handed_off"}, nil
}
