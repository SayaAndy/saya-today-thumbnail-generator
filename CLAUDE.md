# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o saya-today-thumbnail-generator
./saya-today-thumbnail-generator -c config/config-local-unix.sample.json
```

No Makefile. No test files exist yet — `go test ./...` would be the convention.

## What This Is

Concurrent batch thumbnail generator. Reads images from a source (Backblaze B2, S3/S3-compatible, or local filesystem), converts to WebP/JPEG with configurable quality/size, writes to output storage. Maintains a CSV-based file cache to skip already-processed images.

## Architecture

Three pluggable abstractions, each using a factory/registry pattern keyed by string type:

- **InputClient** (`internal/client/input/`) — scans source files, provides readers. Implementations: `b2`, `s3`, `local-unix`
- **OutputClient** (`internal/client/output/`) — writes converted files. Implementations: `b2`, `s3`, `local-unix`
- **Converter** (`internal/converter/`) — decodes image, resizes (Catmull-Rom), encodes to target format. Implementations: `webp`, `jpeg`

Each has a `New*Map` factory function returning a map of type-string → constructor.

## Config System (`config/config.go`)

JSON config with discriminated unions — `"Type"` field selects which struct to unmarshal into. Supports `${ENV_VAR}` expansion in string values. Validated with go-playground/validator.

Key config knobs: `MaxProcessThreads` (conversion concurrency), `MaxPreProcessThreads` (I/O concurrency), `RewriteOn` strategy per converter (`Never`/`UnequalHashInCache`/`Always`).

Sample configs in `config/config-*.sample.json`. Actual configs are gitignored.

## Concurrency Model

Two-tier semaphore system in `main.go`: pre-process (file scanning/reading) and process (image conversion) run with separate thread pool limits. Graceful shutdown on SIGTERM/SIGINT via early-termination flag. Cache map protected by `sync.RWMutex`.

## Environment

Uses direnv (`.envrc`) for credentials (`B2_KEY_ID`, `B2_APPLICATION_KEY`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`). S3 client supports custom endpoints and path-style addressing for S3-compatible stores (MinIO, etc.).
