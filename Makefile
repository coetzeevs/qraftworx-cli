.PHONY: build test test-short lint fmt vet vuln clean run cover-html hooks ci

build:
	CGO_ENABLED=1 go build -trimpath -o bin/qraft ./cmd/qraft

test:
	CGO_ENABLED=1 go test ./... -race -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

test-short:
	CGO_ENABLED=1 go test ./... -race -short

lint:
	golangci-lint run --config=.golangci.yml

fmt:
	gofmt -w .

vet:
	go vet ./...

vuln:
	govulncheck ./...

clean:
	rm -rf bin/ coverage.out

run:
	@test -f .env && export $$(grep -v '^#' .env | xargs) || true; \
	go run ./cmd/qraft $(ARGS)

cover-html: test
	go tool cover -html=coverage.out

hooks:
	pre-commit install
	@echo "Pre-commit hooks installed."

ci: lint test vuln
	@echo "All CI checks passed."
