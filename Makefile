.PHONY: lint
lint:
	./golangci-lint run ./...

.PHONY: test
test: lint
	go test ./...

.PHONY: build
build: lint test
	go build ./cmd/app/main.go

