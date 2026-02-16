// Package infermux implements the InferMux LLM inference router for the
// MIST stack. It routes inference requests to configured providers, tracks
// token usage and cost, and reports trace spans to TokenTrace.
package infermux

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/greynewell/mist-go/protocol"
)

// Provider is an LLM provider that can handle inference requests.
type Provider interface {
	// Name returns the provider identifier (e.g. "openai", "anthropic").
	Name() string

	// Models returns the models this provider supports.
	Models() []string

	// Infer performs inference and returns a response.
	Infer(ctx context.Context, req protocol.InferRequest) (protocol.InferResponse, error)
}

// EchoProvider is a test/development provider that echoes the request back.
// It simulates realistic latency, token counts, and costs.
type EchoProvider struct {
	name   string
	models []string
	delay  time.Duration
}

// NewEchoProvider creates a provider that echoes input for testing.
func NewEchoProvider(name string, models []string, delay time.Duration) *EchoProvider {
	return &EchoProvider{name: name, models: models, delay: delay}
}

func (e *EchoProvider) Name() string    { return e.name }
func (e *EchoProvider) Models() []string { return e.models }

func (e *EchoProvider) Infer(ctx context.Context, req protocol.InferRequest) (protocol.InferResponse, error) {
	select {
	case <-time.After(e.delay):
	case <-ctx.Done():
		return protocol.InferResponse{}, ctx.Err()
	}

	// Build echo content from last message.
	content := "echo: "
	if len(req.Messages) > 0 {
		content += req.Messages[len(req.Messages)-1].Content
	}

	model := req.Model
	if model == "" || model == "auto" {
		if len(e.models) > 0 {
			model = e.models[0]
		}
	}

	tokensIn := int64(0)
	for _, m := range req.Messages {
		tokensIn += int64(len(m.Content) / 4) // rough estimate
	}
	tokensOut := int64(len(content) / 4)
	if tokensOut < 1 {
		tokensOut = 1
	}

	return protocol.InferResponse{
		Model:        model,
		Provider:     e.name,
		Content:      content,
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		CostUSD:      float64(tokensIn+tokensOut) * 0.00001,
		LatencyMS:    e.delay.Milliseconds(),
		FinishReason: "stop",
	}, nil
}

// Registry holds configured providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	modelMap  map[string]string // model name → provider name
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		modelMap:  make(map[string]string),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
	for _, model := range p.Models() {
		r.modelMap[model] = p.Name()
	}
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// Resolve finds the provider for a given model name.
func (r *Registry) Resolve(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct provider lookup.
	if p, ok := r.providers[model]; ok {
		return p, nil
	}

	// Model → provider mapping.
	if provName, ok := r.modelMap[model]; ok {
		if p, ok := r.providers[provName]; ok {
			return p, nil
		}
	}

	// Auto: return first provider.
	if model == "" || model == "auto" {
		for _, p := range r.providers {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no provider for model %q", model)
}

// Providers returns the names of all registered providers.
func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
