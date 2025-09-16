    FROM golang:1.25-alpine AS builder

    WORKDIR /app

    COPY go.mod go.sum ./
    RUN go mod download

    COPY . .

    # Build the binary with optimizations
    RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -ldflags="-w -s -X main.Version=$(git describe --tags --always --dirty) \
        -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
        -a -installsuffix cgo \
        -o /app/auth-server \
        ./services/auth/main.go

    FROM alpine:latest

    WORKDIR /root/

    COPY --from=builder /app/auth-server .

    COPY config.yaml .

    CMD ["./auth-server"]