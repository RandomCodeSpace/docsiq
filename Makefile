.PHONY: build test vet check ui-install ui-build dev-ui dev-go

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X github.com/RandomCodeSpace/docscontext/cmd.Version=$(VERSION) \
            -X github.com/RandomCodeSpace/docscontext/cmd.Commit=$(COMMIT) \
            -X github.com/RandomCodeSpace/docscontext/cmd.Date=$(DATE)

ui-install:
	cd ui && npm install

ui-build:
	cd ui && npm run build

build: ui-build
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" ./...

# Filter out ui/node_modules — a transitive npm dep (flatted) ships a Go package
# that would otherwise be walked by `./...`.
GO_PKGS := $(shell go list ./... 2>/dev/null | grep -v /ui/node_modules/)

test:
	CGO_ENABLED=0 go test -timeout 120s $(GO_PKGS)

vet:
	go vet $(GO_PKGS)

check: build vet test

dev-ui:
	cd ui && npm run dev

dev-go:
	go run . serve
