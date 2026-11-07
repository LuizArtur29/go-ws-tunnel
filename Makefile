.PHONY: build test vet check

build:
	mkdir -p bin
	go build -trimpath -o bin/probe ./cmd/probe
	go build -trimpath -o bin/tunnel ./cmd/tunnel

test:
	go test -race -timeout 90s ./...

vet:
	go vet ./...

check: vet test
	test -z "$$(gofmt -l cmd internal)"
