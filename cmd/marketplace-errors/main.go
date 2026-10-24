package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/marketplace-error-handoff/internal/infrai"
	"example.com/marketplace-error-handoff/internal/marketplace"
)

type ledgerWriter struct{}

func (ledgerWriter) Complete(_ context.Context, handoff marketplace.OrderHandoff) error {
	return errors.New("simulated ledger write failure for order " + handoff.OrderID)
}

type handoffHandler struct {
	service marketplace.Service
}

func (h handoffHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var handoff marketplace.OrderHandoff
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&handoff); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	result, err := h.service.Handoff(r.Context(), handoff)
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	if errors.Is(err, marketplace.ErrAssetNotReady) || errors.Is(err, marketplace.ErrBuyerPending) {
		writeJSON(w, http.StatusConflict, result)
		return
	}
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Error()})
		return
	}
	writeJSON(w, http.StatusBadGateway, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.NewClient(key)
	handler := handoffHandler{service: marketplace.Service{Errors: client, Writer: ledgerWriter{}}}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("marketplace handoff service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
