.PHONY: build clean install release version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.2.0")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/forge/sword/cmd.forgeVersion=$(VERSION) -X github.com/forge/sword/cmd.buildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o forge .

install: build
	cp forge /usr/local/bin/forge

clean:
	rm -f forge

version:
	@echo "forge v$(VERSION) ($(GIT_COMMIT)) built $(BUILD_TIME)"

# Cross-compile for all platforms
release: release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

release-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/forge-linux-amd64 .

release-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/forge-linux-arm64 .

release-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/forge-darwin-amd64 .

release-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/forge-darwin-arm64 .

test:
	go test ./...

run: build
	./forge serve -- claude
