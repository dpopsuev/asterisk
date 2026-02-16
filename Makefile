# Asterisk — build, test, run
# See .cursor/rules/go-testing.mdc for Ginkgo usage.

.PHONY: build test test-ginkgo test-ginkgo-wiring run clean install help

BINARY_NAME := run-mock-flow
CMD_PATH    := ./cmd/run-mock-flow

# Default target
help:
	@echo "Targets:"
	@echo "  make build            - build $(BINARY_NAME)"
	@echo "  make test             - run all tests (go test)"
	@echo "  make test-ginkgo      - run all suites with Ginkgo binary (-r)"
	@echo "  make test-ginkgo-wiring - run only wiring Ginkgo suite"
	@echo "  make run              - run $(BINARY_NAME) with default args"
	@echo "  make clean            - remove built binary"
	@echo "  make install          - install $(BINARY_NAME) to GOPATH/bin"

build:
	go build -o $(BINARY_NAME) $(CMD_PATH)

test:
	go test ./...

# Run Ginkgo specs using the Ginkgo binary (version from go.mod)
test-ginkgo:
	go run github.com/onsi/ginkgo/v2/ginkgo -r

run: build
	./$(BINARY_NAME) -launch 33195 -artifact /tmp/asterisk-artifact.json

clean:
	rm -f $(BINARY_NAME)

install:
	go install $(CMD_PATH)

# Optional: run only wiring Ginkgo suite (no stdlib tests)
test-ginkgo-wiring:
	go run github.com/onsi/ginkgo/v2/ginkgo ./internal/wiring/...
