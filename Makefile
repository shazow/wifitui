BINARY = wifitui

SRCS = %.go
VERSION := $(shell git describe --tags --dirty --always 2> /dev/null || echo "dev")
LDFLAGS = -X main.Version=$(VERSION) -extldflags "-static"

all: $(BINARY)

$(BINARY): *.go
	go build -ldflags "$(LDFLAGS)" .

build: $(BINARY)

clean:
	rm $(BINARY)

run: $(BINARY)
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
