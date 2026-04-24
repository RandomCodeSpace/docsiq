.PHONY: build test test-integration vet check ui-install ui-build test-ui test-ui-coverage dev-ui dev-go

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Version=$(VERSION) \
            -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Commit=$(COMMIT) \
            -X github.com/RandomCodeSpace/docsiq/internal/buildinfo.Date=$(DATE)

ui-install:
	cd ui && npm install

ui-build:
	cd ui && npm run build

build: ui-build
	CGO_ENABLED=1 go build -tags "sqlite_fts5" -ldflags "$(LDFLAGS)" -o docsiq .

# Filter out ui/node_modules — a transitive npm dep (flatted) ships a Go package
# that would otherwise be walked by `./...`.
GO_PKGS := $(shell CGO_ENABLED=1 go list -tags "sqlite_fts5" ./... 2>/dev/null | grep -v /ui/node_modules/)

test:
	CGO_ENABLED=1 go test -tags "sqlite_fts5" -timeout 300s $(GO_PKGS)

test-integration:
	CGO_ENABLED=1 go test -tags "sqlite_fts5 integration" -timeout 600s $(GO_PKGS)

vet:
	CGO_ENABLED=1 go vet -tags "sqlite_fts5" $(GO_PKGS)

test-ui:
	cd ui && npm test -- --run

test-ui-coverage:
	cd ui && npm run test:coverage

check: build vet test test-ui

dev-ui:
	cd ui && npm run dev

dev-go:
	go run . serve
