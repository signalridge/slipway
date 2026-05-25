# syntax=docker/dockerfile:1

FROM golang:1.25-alpine@sha256:8d22e29d960bc50cd025d93d5b7c7d220b1ee9aa7a239b3c8f55a57e987e8d45 AS builder

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

FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639

COPY --from=builder /out/slipway /slipway
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nonroot:nonroot
ENTRYPOINT ["/slipway"]
CMD ["--help"]

LABEL org.opencontainers.image.title="slipway"
LABEL org.opencontainers.image.description="Governance CLI for AI-assisted software delivery"
LABEL org.opencontainers.image.source="https://github.com/signalridge/slipway"
LABEL org.opencontainers.image.vendor="SignalRidge"
