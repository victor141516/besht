BIN       := besht
BUILD_DIR := ./dist
GO        := go
PKGS      := ./internal/...

.PHONY: all build test cover cover-html clean

all: build

build:
	$(GO) build -o $(BUILD_DIR)/$(BIN) ./cmd/besht/

test:
	$(GO) test $(PKGS)

cover:
	$(GO) test -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -func=coverage.out

cover-html: cover
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
