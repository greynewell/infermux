# infermux

Inference router. Part of the [MIST stack](https://github.com/greynewell/mist-go).

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Install

```bash
go get github.com/greynewell/infermux
```

## Provider interface

```go
type Provider interface {
    Name() string
    Models() []string
    Infer(ctx context.Context, req protocol.InferRequest) (protocol.InferResponse, error)
}
```

## Route

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

Tracks tokens and cost per request. Reports spans to [TokenTrace](https://github.com/greynewell/tokentrace).

## HTTP API

```go
handler := infermux.NewHandler(router, reg)
http.HandleFunc("POST /mist", handler.Ingest)
http.HandleFunc("POST /infer", handler.InferDirect)
http.HandleFunc("GET /providers", handler.Providers)
```

## CLI

```bash
infermux serve --addr :8600 --tokentrace http://localhost:8700
infermux infer --model echo-v1 --prompt "Hello world"
```
