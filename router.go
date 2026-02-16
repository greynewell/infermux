package infermux

import (
	"context"
	"fmt"
	"time"

	"github.com/greynewell/mist-go/protocol"
	"github.com/greynewell/mist-go/trace"
	"github.com/greynewell/mist-go/tokentrace"
)

// Router routes inference requests to the appropriate provider and
// reports trace spans to TokenTrace.
type Router struct {
	registry *Registry
	reporter *tokentrace.Reporter
}

// NewRouter creates a router with the given provider registry and trace reporter.
func NewRouter(reg *Registry, reporter *tokentrace.Reporter) *Router {
	return &Router{registry: reg, reporter: reporter}
}

// Infer routes a request to the appropriate provider, instruments the
// call with tracing, and returns the response.
func (r *Router) Infer(ctx context.Context, req protocol.InferRequest) (protocol.InferResponse, error) {
	ctx, span := trace.Start(ctx, "infermux.infer")

	provider, err := r.registry.Resolve(req.Model)
	if err != nil {
		span.SetAttr("error", err.Error())
		span.End("error")
		r.reporter.Report(ctx, span)
		return protocol.InferResponse{}, err
	}

	span.SetAttr("provider", provider.Name())
	span.SetAttr("model", req.Model)

	start := time.Now()
	resp, err := provider.Infer(ctx, req)
	latency := time.Since(start)

	if err != nil {
		span.SetAttr("error", err.Error())
		span.End("error")
		r.reporter.Report(ctx, span)
		return protocol.InferResponse{}, fmt.Errorf("provider %s: %w", provider.Name(), err)
	}

	span.SetAttr("tokens_in", float64(resp.TokensIn))
	span.SetAttr("tokens_out", float64(resp.TokensOut))
	span.SetAttr("cost_usd", resp.CostUSD)
	span.SetAttr("latency_ms", latency.Milliseconds())
	span.SetAttr("finish_reason", resp.FinishReason)
	span.End("ok")

	r.reporter.Report(ctx, span)
	return resp, nil
}
