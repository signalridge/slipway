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
    rm -rf dist/ coverage.txt coverage.html website/dist/ website/.astro/ tmp/

[group('test')]
test:
    go test -timeout=20m ./... -count=1

[group('test')]
test-race:
    go test -timeout=20m ./... -race -count=1

[group('test')]
acceptance: build-bin
    SLIPWAY_BIN="$PWD/slipway" ./acceptance/machine-protocol.sh
    SLIPWAY_BIN="$PWD/slipway" ./acceptance/adapters.sh

[group('test')]
coverage:
    go test -timeout=20m ./... -coverprofile=coverage.txt -count=1
    go tool cover -func=coverage.txt

[group('test')]
coverage-gate mode="check":
    mkdir -p tmp
    set -eu; \
    if [ "{{mode}}" = write ] && [ "$(go env GOOS)" != linux ]; then echo 'coverage baselines must be written on linux' >&2; exit 1; fi; \
    gated_packages="$(go list \
        ./cmd \
        ./internal/adapter \
        ./internal/autopilot \
        ./internal/fsutil \
        ./internal/jsonstrict \
        ./internal/recoverycmd \
        ./internal/runstore \
        ./internal/testlint \
        ./internal/tmpl | paste -sd, -)"; \
    go test -timeout=20m ./... -count=1 \
        -coverpkg="$gated_packages" \
        -coverprofile=tmp/coverage-gated.out; \
    go run ./internal/covergate/cmd/covergate \
        -{{mode}} -profile=tmp/coverage-gated.out -baseline=coverage-baseline.json

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
