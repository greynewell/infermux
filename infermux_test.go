package infermux

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greynewell/mist-go/protocol"
	"github.com/greynewell/mist-go/tokentrace"
)

func echoRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(NewEchoProvider("echo", []string{"echo-v1", "echo-v2"}, time.Millisecond))
	return reg
}

func testRouter() *Router {
	return NewRouter(echoRegistry(), tokentrace.NewReporter("infermux", ""))
}

func testHandler() *Handler {
	reg := echoRegistry()
	router := NewRouter(reg, tokentrace.NewReporter("infermux", ""))
	return NewHandler(router, reg)
}

// --- Provider tests ---

func TestEchoProviderName(t *testing.T) {
	p := NewEchoProvider("test", []string{"m1"}, 0)
	if p.Name() != "test" {
		t.Errorf("Name = %s, want test", p.Name())
	}
}

func TestEchoProviderModels(t *testing.T) {
	p := NewEchoProvider("test", []string{"m1", "m2"}, 0)
	if len(p.Models()) != 2 {
		t.Errorf("Models = %d, want 2", len(p.Models()))
	}
}

func TestEchoProviderInfer(t *testing.T) {
	p := NewEchoProvider("test", []string{"m1"}, time.Millisecond)
	resp, err := p.Infer(context.Background(), protocol.InferRequest{
		Model:    "m1",
		Messages: []protocol.ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "test" {
		t.Errorf("Provider = %s, want test", resp.Provider)
	}
	if resp.Content != "echo: hello" {
		t.Errorf("Content = %s, want 'echo: hello'", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("FinishReason = %s, want stop", resp.FinishReason)
	}
}

func TestEchoProviderContextCancel(t *testing.T) {
	p := NewEchoProvider("test", []string{"m1"}, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Infer(ctx, protocol.InferRequest{Model: "m1"})
	if err == nil {
		t.Error("expected context cancelled error")
	}
}

// --- Registry tests ---

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	p := NewEchoProvider("openai", []string{"gpt-4"}, 0)
	reg.Register(p)

	got, ok := reg.Get("openai")
	if !ok {
		t.Fatal("expected to find openai provider")
	}
	if got.Name() != "openai" {
		t.Errorf("Name = %s, want openai", got.Name())
	}
}

func TestRegistryResolveByModel(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewEchoProvider("anthropic", []string{"claude-3-opus", "claude-sonnet-4-5-20250929"}, 0))

	p, err := reg.Resolve("claude-3-opus")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("resolved to %s, want anthropic", p.Name())
	}
}

func TestRegistryResolveAuto(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewEchoProvider("default", []string{"m1"}, 0))

	p, err := reg.Resolve("auto")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Error("expected a provider for auto")
	}
}

func TestRegistryResolveUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Resolve("unknown-model")
	if err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestRegistryProviders(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewEchoProvider("a", nil, 0))
	reg.Register(NewEchoProvider("b", nil, 0))

	names := reg.Providers()
	if len(names) != 2 {
		t.Errorf("Providers = %d, want 2", len(names))
	}
}

// --- Router tests ---

func TestRouterInfer(t *testing.T) {
	router := testRouter()
	resp, err := router.Infer(context.Background(), protocol.InferRequest{
		Model:    "echo-v1",
		Messages: []protocol.ChatMessage{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "echo: test" {
		t.Errorf("Content = %s, want 'echo: test'", resp.Content)
	}
}

func TestRouterInferUnknownModel(t *testing.T) {
	router := testRouter()
	_, err := router.Infer(context.Background(), protocol.InferRequest{
		Model: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestRouterInferAuto(t *testing.T) {
	router := testRouter()
	resp, err := router.Infer(context.Background(), protocol.InferRequest{
		Model:    "auto",
		Messages: []protocol.ChatMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Provider != "echo" {
		t.Errorf("Provider = %s, want echo", resp.Provider)
	}
}

// --- Handler tests ---

func TestHandlerIngestSuccess(t *testing.T) {
	h := testHandler()
	msg, _ := protocol.New("test", protocol.TypeInferRequest, protocol.InferRequest{
		Model:    "echo-v1",
		Messages: []protocol.ChatMessage{{Role: "user", Content: "hello"}},
	})
	body, _ := msg.Marshal()

	req := httptest.NewRequest("POST", "/mist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var respMsg protocol.Message
	if err := json.Unmarshal(w.Body.Bytes(), &respMsg); err != nil {
		t.Fatal(err)
	}
	if respMsg.Type != protocol.TypeInferResponse {
		t.Errorf("type = %s, want infer.response", respMsg.Type)
	}
}

func TestHandlerIngestWrongType(t *testing.T) {
	h := testHandler()
	msg, _ := protocol.New("test", protocol.TypeHealthPing, protocol.HealthPing{From: "test"})
	body, _ := msg.Marshal()

	req := httptest.NewRequest("POST", "/mist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlerIngestBadJSON(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest("POST", "/mist", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandlerInferDirect(t *testing.T) {
	h := testHandler()
	body, _ := json.Marshal(protocol.InferRequest{
		Model:    "echo-v1",
		Messages: []protocol.ChatMessage{{Role: "user", Content: "direct test"}},
	})

	req := httptest.NewRequest("POST", "/infer", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.InferDirect(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp protocol.InferResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Content != "echo: direct test" {
		t.Errorf("Content = %s, want 'echo: direct test'", resp.Content)
	}
}

func TestHandlerProviders(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest("GET", "/providers", nil)
	w := httptest.NewRecorder()
	h.Providers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp ProvidersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(resp.Providers))
	}
	if resp.Providers[0].Name != "echo" {
		t.Errorf("provider name = %s, want echo", resp.Providers[0].Name)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest("GET", "/mist", nil)
	w := httptest.NewRecorder()
	h.Ingest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestInferFromCLI(t *testing.T) {
	router := testRouter()
	resp, err := InferFromCLI(context.Background(), router, "echo-v1", "cli test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "echo: cli test" {
		t.Errorf("Content = %s, want 'echo: cli test'", resp.Content)
	}
}
