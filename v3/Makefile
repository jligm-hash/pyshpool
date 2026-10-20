VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/jligm-hash/pyshpool/v3/internal/version.Version=$(VERSION) \
	-X github.com/jligm-hash/pyshpool/v3/internal/version.Commit=$(COMMIT) \
	-X github.com/jligm-hash/pyshpool/v3/internal/version.Date=$(DATE)

.PHONY: all test vet fmt build clean docs serve bench

all: test build

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/pysh ./cmd/pysh
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/pysh-agent ./cmd/pysh-agent

docs:
	mkdocs build --strict

serve:
	mkdocs serve

bench: build
	python3 scripts/bench.py

clean:
	rm -rf dist
