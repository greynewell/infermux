# infermux

Inference routing across LLM providers for the MIST stack. Route requests to the right model, track tokens and cost, report traces.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

InferMux routes inference requests to configured providers, tracks token usage and cost, and reports trace spans to TokenTrace.

- **Provider registry** — register any backend implementing the Provider interface
- **Model routing** — resolve models to providers automatically
- **Token & cost tracking** — per-request token counts and USD cost
- **Trace integration** — every inference call produces a trace span
- **HTTP API** — ingest via MIST protocol or direct JSON

## Install

```bash
go get github.com/greynewell/infermux
```

## Usage

### Start the server

```bash
go run ./cmd/infermux serve --addr :8600
go run ./cmd/infermux serve --addr :8600 --tokentrace http://localhost:8700
```

### One-shot inference

```bash
go run ./cmd/infermux infer --model echo-v1 --prompt "Hello world"
```

### Provider interface

```go
type Provider interface {
    Name() string
    Models() []string
    Infer(ctx context.Context, req protocol.InferRequest) (protocol.InferResponse, error)
}
```

### Register providers and route

```go
reg := infermux.NewRegistry()
reg.Register(myOpenAIProvider)
reg.Register(myAnthropicProvider)

reporter := tokentrace.NewReporter("infermux", "http://localhost:8700")
router := infermux.NewRouter(reg, reporter)

resp, err := router.Infer(ctx, protocol.InferRequest{
    Model:    "claude-sonnet-4-5-20250929",
    Messages: []protocol.ChatMessage{{Role: "user", Content: "Hello"}},
})
```

### HTTP API

```go
handler := infermux.NewHandler(router, reg)
http.HandleFunc("POST /mist", handler.Ingest)         // MIST protocol
http.HandleFunc("POST /infer", handler.InferDirect)    // Direct JSON
http.HandleFunc("GET /providers", handler.Providers)   // List providers
```

## Part of the MIST stack

| Tool | Purpose |
|------|---------|
| **MatchSpec** | Evaluation framework |
| **InferMux** | Inference routing (this repo) |
| **SchemaFlux** | Structured data compiler |
| **TokenTrace** | Token-level observability |

Shared foundation: [mist-go](https://github.com/greynewell/mist-go)

## License

MIT — see [LICENSE](LICENSE) for details.

---

Built by [Grey Newell](https://greynewell.com) | [infermux.dev](https://infermux.dev)
