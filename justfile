default:
    @just --list

[group('build')]
build:
    go build ./...

[group('build')]
build-bin:
    go build -o slipway .

[group('build')]
build-release version="dev" commit="unknown":
    go build -ldflags "-s -w \
        -X github.com/signalridge/slipway/cmd.version={{version}} \
        -X github.com/signalridge/slipway/cmd.commit={{commit}} \
        -X github.com/signalridge/slipway/cmd.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        -o slipway .

[group('build')]
clean:
    rm -f slipway
    rm -rf dist/ coverage.txt coverage.html site/ tmp/

[group('test')]
test:
    go test -timeout=20m ./... -count=1

[group('test')]
test-race:
    go test -timeout=20m ./... -race -count=1

[group('test')]
coverage:
    go test -timeout=20m ./... -coverprofile=coverage.txt -count=1
    go tool cover -func=coverage.txt

# Run the governed coverage gates locally (fails closed on regression).
[group('test')]
coverage-gate:
    mkdir -p tmp
    go test -timeout=20m ./... -count=1 \
        -coverpkg="$(go list ./internal/engine/gate ./internal/engine/governance ./internal/engine/progression ./cmd ./internal/state | paste -sd, -)" \
        -coverprofile=tmp/coverage-gated.out
    go run ./internal/coverage/cmd/covergate -target kernel -check -profile tmp/coverage-gated.out
    go run ./internal/coverage/cmd/covergate -target public-surface -check -profile tmp/coverage-gated.out

# Ratchet committed governed coverage baselines to current (review the diff).
[group('test')]
coverage-baseline:
    mkdir -p tmp
    go test -timeout=20m ./... -count=1 \
        -coverpkg="$(go list ./internal/engine/gate ./internal/engine/governance ./internal/engine/progression ./cmd ./internal/state | paste -sd, -)" \
        -coverprofile=tmp/coverage-gated.out
    go run ./internal/coverage/cmd/covergate -target kernel -write -profile tmp/coverage-gated.out
    go run ./internal/coverage/cmd/covergate -target public-surface -write -profile tmp/coverage-gated.out

[group('lint')]
lint:
    go vet ./...
    golangci-lint run --timeout 5m

[group('lint')]
lint-full:
    golangci-lint run --timeout 5m

[group('format')]
fmt:
    gofmt -w .

[group('release')]
release-check:
    goreleaser check

[group('release')]
release-dry:
    goreleaser release --snapshot --clean

[group('docker')]
docker-build version="dev":
    docker build \
        --build-arg VERSION={{version}} \
        --build-arg COMMIT=$(git rev-parse --short HEAD) \
        --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
        -t slipway:{{version}} .

[group('docker')]
docker-run version="dev" *args:
    docker run --rm slipway:{{version}} {{args}}

[group('deps')]
deps:
    go mod download

[group('deps')]
tidy:
    go mod tidy

[group('deps')]
vuln:
    govulncheck ./...
