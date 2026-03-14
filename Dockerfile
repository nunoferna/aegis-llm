# STAGE 1: Builder
FROM golang:1.26-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o aegis-gateway ./cmd/aegis

# STAGE 2: Runner
FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1

LABEL org.opencontainers.image.title="aegis-llm" \
    org.opencontainers.image.description="LLM Gateway with semantic caching and rate limiting" \
    org.opencontainers.image.source="https://github.com/nunoferna/aegis-llm" \
    org.opencontainers.image.licenses="Apache-2.0"

ENV TMPDIR=/tmp
VOLUME ["/tmp"]

WORKDIR /

COPY --from=builder --chown=nonroot:nonroot /app/aegis-gateway /aegis-gateway

EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/aegis-gateway"]