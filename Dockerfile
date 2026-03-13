# STAGE 1: Builder
# Use the official Golang image to build the binary
FROM golang:1.26-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files first (for efficient layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary securely
# CGO_ENABLED=0 ensures it's a static binary with no external C dependencies
# -ldflags="-w -s" strips debugging info to make the binary incredibly small
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o aegis-gateway ./cmd/aegis

# STAGE 2: Runner
# Use a distroless or alpine image for a tiny, secure production footprint
FROM alpine:latest

# Add CA certificates so the proxy can securely call HTTPS APIs (like OpenAI)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/aegis-gateway .

# Expose our proxy and metrics port
EXPOSE 8080

# Command to run the executable
CMD ["./aegis-gateway"]