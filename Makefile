.PHONY: lint test test-coverage test-swiftbar build fmt vet check clean install

SHELL := /bin/bash

BINARY := fleet
BUILD_DIR := bin

lint:
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/fleet/

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

test-swiftbar:
	@diff -u scripts/swiftbar/fixtures/status.expected.txt \
		<(FLEET_STATUS_FIXTURE=scripts/swiftbar/fixtures/status.json \
			./scripts/swiftbar/fleet.10s.sh)
	@echo "swiftbar plugin output matches golden."

check: fmt lint vet test test-swiftbar build

clean:
	rm -rf $(BUILD_DIR)/ coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)
