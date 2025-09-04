.PHONY: proto build run test docker

proto:
	@echo "Generating proto files..."
	protoc --go_out=. --go-grpc_out=. proto/*.proto

build:
	@echo "Building services..."
	go build -o bin/auth-service services/auth-service/main.go

run-dev:
	docker-compose -f docker-compose.dev.yml up --build

test:
	go test ./...

docker-build:
	docker build -t auth-service services/auth-service/