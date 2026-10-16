.PHONY: build test test-live lint run

build:
	go build -o brewery ./cmd/brewery

test:
	go test ./...

test-live:
	go test -tags live -count=1 ./internal/brew/

# golangci-lint v2: brew install golangci-lint
lint:
	golangci-lint run ./...

run: build
	./brewery
