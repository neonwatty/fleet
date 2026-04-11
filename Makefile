.PHONY: lint test test-coverage build fmt vet check clean install

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

check: fmt lint vet test build

clean:
	rm -rf $(BUILD_DIR)/ coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)
