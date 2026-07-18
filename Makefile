BINARY = wifitui

VERSION := $(shell git describe --tags --dirty --always 2> /dev/null || echo "dev")
# Portable builds disable cgo and retain the static external-link preference.
LDFLAGS = -X main.Version=$(VERSION) -extldflags "-static"
# CoreWLAN requires cgo and Apple's dynamically linked system frameworks.
DARWIN_LDFLAGS = -X main.Version=$(VERSION)

.PHONY: all build build-static build-darwin run

all: build

build: build-static

build-static:
	CGO_ENABLED=0 go build -o $(BINARY) -ldflags "$(LDFLAGS)" .

build-darwin:
	CGO_ENABLED=1 GOOS=darwin go build -o $(BINARY) -ldflags "$(DARWIN_LDFLAGS)" .

clean:
	rm $(BINARY)

run:
	go run .

mock:
	go run -tags mock .

test:
	go test -v -test.timeout 5s ./...

vendorHash: flake.nix
flake.nix: go.sum
	go mod vendor
	sed -i "s|vendorHash = \".*\"|vendorHash = \"$$(nix hash path vendor)\"|" flake.nix;
	rm -rf ./vendor
