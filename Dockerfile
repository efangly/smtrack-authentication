FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

RUN go mod download

# Copy source code
COPY . .

# Build the binary
# The service plus the two operator tools, which run as Jobs from the same image.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o auth-service . && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o import-legacy ./cmd/import-legacy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o purge ./cmd/purge

# Final stage
FROM alpine:3.23.3

WORKDIR /app

RUN apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/auth-service /app/import-legacy /app/purge ./

# Run unprivileged.
RUN adduser -D -u 10001 appuser && chown appuser /app/auth-service /app/import-legacy /app/purge
USER appuser

ENV TZ=Asia/Bangkok

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -q --spider http://localhost:8080/auth/health || exit 1

EXPOSE 8080

CMD ["./auth-service"]
