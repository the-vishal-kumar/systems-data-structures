GO = docker run --rm -v "$(CURDIR)":/src -w /src golang:1.27.3-bookworm
.PHONY: demo test
demo: ## run every demo (no database needed)
	$(GO) sh -c 'go run ./cmd/hashdemo && go run ./cmd/filterdemo'
test:
	$(GO) go test -race -count=1 ./...
