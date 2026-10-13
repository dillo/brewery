.PHONY: build test test-live run

build:
	go build -o brewery ./cmd/brewery

test:
	go test ./...

test-live:
	go test -tags live -count=1 ./internal/brew/

run: build
	./brewery
