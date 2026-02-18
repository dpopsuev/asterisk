# Asterisk — build, test, run
# See .cursor/rules/go-testing.mdc for Ginkgo usage.

.PHONY: build build-all test test-ginkgo test-ginkgo-wiring run clean install help

BIN_DIR     := bin
CMD_ASTERISK := ./cmd/asterisk
CMD_ANALYZE_RP := ./cmd/asterisk-analyze-rp-cursor
CMD_MOCK     := ./cmd/run-mock-flow

# Default target
help:
	@echo "Targets:"
	@echo "  make build            - build asterisk CLI to $(BIN_DIR)/"
	@echo "  make build-all        - build all binaries to $(BIN_DIR)/"
	@echo "  make test             - run all tests (go test)"
	@echo "  make test-ginkgo      - run all suites with Ginkgo binary (-r)"
	@echo "  make test-ginkgo-wiring - run only wiring Ginkgo suite"
	@echo "  make run              - build and run asterisk with default args"
	@echo "  make clean            - remove $(BIN_DIR)/"
	@echo "  make install          - install asterisk to GOPATH/bin"

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	go build -o $(BIN_DIR)/asterisk $(CMD_ASTERISK)
	go build -o $(BIN_DIR)/asterisk-analyze-rp-cursor $(CMD_ANALYZE_RP)

build-all: $(BIN_DIR)
	go build -o $(BIN_DIR)/asterisk $(CMD_ASTERISK)
	go build -o $(BIN_DIR)/asterisk-analyze-rp-cursor $(CMD_ANALYZE_RP)
	go build -o $(BIN_DIR)/run-mock-flow $(CMD_MOCK)

test:
	go test ./...

# Run Ginkgo specs using the Ginkgo binary (version from go.mod)
test-ginkgo:
	go run github.com/onsi/ginkgo/v2/ginkgo -r

run: build
	./$(BIN_DIR)/asterisk analyze -launch 33195 -o /tmp/asterisk-artifact.json

clean:
	rm -rf $(BIN_DIR)

install:
	go install $(CMD_ASTERISK)

# Optional: run only wiring Ginkgo suite (no stdlib tests)
test-ginkgo-wiring:
	go run github.com/onsi/ginkgo/v2/ginkgo ./internal/wiring/...
