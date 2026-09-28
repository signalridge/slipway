# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine3.24@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X github.com/signalridge/slipway/cmd.version=${VERSION} \
    -X github.com/signalridge/slipway/cmd.commit=${COMMIT} \
    -X github.com/signalridge/slipway/cmd.date=${DATE}" \
    -o /out/slipway .

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache ca-certificates git \
    && git config --system --add safe.directory '*' \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S nonroot -G nonroot

COPY --from=builder /out/slipway /slipway

USER nonroot:nonroot
ENTRYPOINT ["/slipway"]
CMD ["--help"]

LABEL org.opencontainers.image.title="slipway"
LABEL org.opencontainers.image.description="User-controlled soft autopilot for AI coding"
LABEL org.opencontainers.image.source="https://github.com/signalridge/slipway"
LABEL org.opencontainers.image.vendor="SignalRidge"
LABEL org.opencontainers.image.licenses="BSD-3-Clause"
