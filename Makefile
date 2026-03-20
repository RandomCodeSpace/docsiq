.PHONY: build test vet check ui-install ui-build dev-ui dev-go

ui-install:
	cd ui && npm install

ui-build:
	cd ui && npm run build

build: ui-build
	CGO_ENABLED=0 go build ./...

test:
	CGO_ENABLED=0 go test -timeout 120s ./...

vet:
	go vet ./...

check: build vet test

dev-ui:
	cd ui && npm run dev

dev-go:
	go run . serve
