# QCS Cargo — PRD 13.1 build commands
.PHONY: build run migrate test lint wasm deps

deps:
	go mod download
	go mod tidy

build:
	go build -o qcs-server ./cmd/server
	go build -o qcs-migrate ./cmd/migrate

run: build
	./qcs-server

migrate: build
	./qcs-migrate

test:
	go test ./cmd/... ./internal/... -race -cover

lint:
	golangci-lint run ./cmd/... ./internal/...

# Copy Go's WASM loader and build frontend to web/
wasm:
	cp "$$(go env GOROOT)/misc/wasm/wasm_exec.js" web/
	GOOS=js GOARCH=wasm go build -o web/app.wasm ./frontend

# Generate sqlc code (requires sqlc: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)
sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
