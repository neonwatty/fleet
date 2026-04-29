.PHONY: lint test test-coverage build dist fmt vet check clean install

SHELL := /bin/bash

BINARY := fleet
BUILD_DIR := bin
DIST_DIR := dist
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
DIST_NAME := $(BINARY)_$(VERSION)_darwin_arm64

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/fleet/

dist: build
	mkdir -p $(DIST_DIR)/$(DIST_NAME)
	cp $(BUILD_DIR)/$(BINARY) $(DIST_DIR)/$(DIST_NAME)/
	cp README.md LICENSE $(DIST_DIR)/$(DIST_NAME)/ 2>/dev/null || true
	tar -C $(DIST_DIR) -czf $(DIST_DIR)/$(DIST_NAME).tar.gz $(DIST_NAME)

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

check: fmt lint vet test menubar-test build

clean:
	rm -rf $(BUILD_DIR)/ $(DIST_DIR)/ coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

.PHONY: menubar-build menubar-test menubar-install menubar-install-login menubar-clean

menubar-build:
	cd menubar && xcodegen generate && xcodebuild build \
	  -project FleetMenuBar.xcodeproj -scheme FleetMenuBar \
	  -configuration Release -destination 'platform=macOS' \
	  -derivedDataPath build

menubar-test:
	cd menubar && xcodegen generate && xcodebuild test \
	  -project FleetMenuBar.xcodeproj -scheme FleetMenuBar \
	  -destination 'platform=macOS' \
	  -derivedDataPath build

menubar-install: menubar-build
	mkdir -p $(HOME)/Applications
	rm -rf $(HOME)/Applications/FleetMenuBar.app
	cp -R menubar/build/Build/Products/Release/FleetMenuBar.app $(HOME)/Applications/
	@if [ -f "$(HOME)/Library/LaunchAgents/com.neonwatty.FleetMenuBar.plist" ]; then \
	  echo "LaunchAgent detected — restarting managed instance"; \
	  launchctl kickstart -k "gui/$$(id -u)/com.neonwatty.FleetMenuBar"; \
	else \
	  open $(HOME)/Applications/FleetMenuBar.app; \
	fi

menubar-install-login:
	./menubar/scripts/install-login-item.sh

menubar-clean:
	rm -rf menubar/FleetMenuBar.xcodeproj menubar/build
