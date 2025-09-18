    FROM golang:1.25-alpine AS builder

    WORKDIR /app
    
    COPY go.mod go.sum ./
    RUN go mod download
    
    COPY . .
    
    # Build the binary with optimizations
    RUN --mount=type=cache,target=/root/.cache/go-build \
        --mount=type=cache,target=/go/pkg/mod \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -ldflags="-w -s \
            -X main.Version=$VERSION \
            -X main.BuildTime=$BUILD_TIME" \
        -a -installsuffix cgo \
        -o /app/gateway-server \
        ./services/api-gateway/main.go
    
    FROM alpine:latest
    
    WORKDIR /root/
    
    COPY --from=builder /app/gateway-server .

    COPY config.yaml .
    
    CMD ["./gateway-server"]