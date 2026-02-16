package infermux

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/greynewell/mist-go/protocol"
)

// Handler provides HTTP handlers for the InferMux API.
type Handler struct {
	router   *Router
	registry *Registry
}

// NewHandler creates a handler wired to the given router and registry.
func NewHandler(router *Router, registry *Registry) *Handler {
	return &Handler{router: router, registry: registry}
}

// Ingest handles POST /mist — accepts MIST protocol messages containing
// inference requests and returns inference responses.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg protocol.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "invalid message: "+err.Error(), http.StatusBadRequest)
		return
	}

	if msg.Type != protocol.TypeInferRequest {
		http.Error(w, "expected type infer.request, got "+msg.Type, http.StatusBadRequest)
		return
	}

	var req protocol.InferRequest
	if err := msg.Decode(&req); err != nil {
		http.Error(w, "invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.router.Infer(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	respMsg, err := protocol.New(protocol.SourceInferMux, protocol.TypeInferResponse, resp)
	if err != nil {
		http.Error(w, "response marshal: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respMsg)
}

// InferDirect handles POST /infer — accepts a direct InferRequest JSON body
// (without the MIST envelope) for simpler integration.
func (h *Handler) InferDirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.InferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.router.Infer(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ProvidersResponse is the JSON body for GET /providers.
type ProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

// ProviderInfo describes a registered provider.
type ProviderInfo struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

// Providers handles GET /providers — lists all registered providers.
func (h *Handler) Providers(w http.ResponseWriter, r *http.Request) {
	var resp ProvidersResponse
	for _, name := range h.registry.Providers() {
		if p, ok := h.registry.Get(name); ok {
			resp.Providers = append(resp.Providers, ProviderInfo{
				Name:   p.Name(),
				Models: p.Models(),
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// InferFromCLI performs a one-shot inference from CLI arguments.
func InferFromCLI(ctx context.Context, router *Router, model, prompt string) (protocol.InferResponse, error) {
	req := protocol.InferRequest{
		Model: model,
		Messages: []protocol.ChatMessage{
			{Role: "user", Content: prompt},
		},
	}
	return router.Infer(ctx, req)
}
