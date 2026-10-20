#!/usr/bin/env bash
# Build self-contained pysh and pysh-agent binaries.
#
# Examples:
#   ./scripts/build.sh
#   VERSION=v3.0.0 ./scripts/build.sh
#   GOOS=linux GOARCH=amd64 ./scripts/build.sh
set -euo pipefail

cd "$(dirname "$0")/.."

version="${VERSION:-dev}"
commit="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
ldflags="-s -w \
  -X github.com/jligm-hash/pyshpool/v3/internal/version.Version=${version} \
  -X github.com/jligm-hash/pyshpool/v3/internal/version.Commit=${commit} \
  -X github.com/jligm-hash/pyshpool/v3/internal/version.Date=${date}"

mkdir -p dist
export CGO_ENABLED=0
go build -trimpath -ldflags "$ldflags" -o dist/pysh ./cmd/pysh
go build -trimpath -ldflags "$ldflags" -o dist/pysh-agent ./cmd/pysh-agent

echo "built:"
ls -lh dist/pysh dist/pysh-agent
