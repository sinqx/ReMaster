
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/auth-server ./services/auth/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/auth-server .

COPY ../config.yaml /app/config.yaml

CMD ["./auth-server"]