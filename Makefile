UNPACKERR_DIR ?= ../unpackerr
UNPACKERR_BIN ?= $(abspath $(UNPACKERR_DIR)/unpackerr)

.PHONY: all test test-unit faker inject unpackerr lint

all: test-unit

unpackerr:
	@if { [ -n "$$UNPACKERR_BIN" ] && [ -x "$$UNPACKERR_BIN" ]; } || [ -x "$(UNPACKERR_BIN)" ]; then \
		exit 0; \
	fi; \
	cd "$(UNPACKERR_DIR)" && go build -o unpackerr .

test-unit:
	go test -race -count=1 ./internal/...

test: unpackerr
	UNPACKERR_BIN="$(UNPACKERR_BIN)" go test -tags=integration -timeout 3m -count=1 -v ./test/...

faker:
	go build -o faker ./cmd/faker

inject:
	go build -o inject ./cmd/inject

lint:
	golangci-lint run ./...
