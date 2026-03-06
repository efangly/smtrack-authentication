FROM --platform=linux/amd64 golang:1.25-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o auth-service .

# Final stage
FROM --platform=linux/amd64 alpine:3.23.3

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/auth-service .

ENV TZ=Asia/Bangkok

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -q --spider http://localhost:8080/auth/health || exit 1

EXPOSE 8080

CMD ["./auth-service"]
