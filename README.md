```
ReMaster/
├── services/
│   ├── api-gateway/
│   │   ├── main.go
│   │   ├── handlers/
│   │   └── middleware/
│   ├── auth-service/
│   │   ├── main.go
│   │   ├── handlers/
│   │   ├── models/
│   │   ├── repositories/
│   │   └── services/
│   ├── user-service/
│   ├── order-service/
│   ├── review-service/
│   ├── media-service/
│   ├── chat-service/
│   └── notification-service/
│
├── proto/
│   ├── auth.proto
│   ├── common.proto
│   ├── user.proto
│   ├── order.proto
│   ├── review.proto
│   ├── media.proto
│   └── chat.proto
│
├── infrastructure/
│   ├── docker/
│   │   └── docker-compose.dev.yml
│   ├── k8s/
│   │   ├── auth-service.yaml
│   │   ├── user-service.yaml
│   │   ├── postgres.yaml
│   │   ├── redis.yaml
│   │   └── kafka.yaml
│   ├── monitoring/
│   │   ├── prometheus.yml
│   │   └── grafana/
│   └── aws/
│
├── shared/
│   ├── config.go
│   ├── connection/
│   │   ├── aws.go
│   │   ├── kafka.go
│   │   ├── mongoDB.go
│   │   └── redis.go
│   │
│   ├── utils/
│   └── middleware/
│
├── Tiltfile
├── Makefile
├── .env.example
├── go.mod
├── go.sum
└── README.md

```

This project is designed to maximize practice with modern tools in the Go ecosystem. Below is the chosen stack, with short descriptions and useful learning resources.

---

### 🚪 API Gateway & HTTP

- **Gin (gateway)** — web framework for API Gateway.
  🔗 [Gin Web Framework](https://gin-gonic.com/docs/)
  🔗 [Gin on GitHub](https://github.com/gin-gonic/gin)
- **grpc-go (inter-service communication)** — gRPC for microservices.
  🔗 [gRPC-Go](https://grpc.io/docs/languages/go/)
  🔗 [grpc-go GitHub](https://github.com/grpc/grpc-go)
- **grpc-gateway (optional)** — auto-generate REST endpoints from gRPC services.
  🔗 [grpc-gateway](https://grpc-ecosystem.github.io/grpc-gateway/)
- **swaggo (API docs)** — generate Swagger/OpenAPI docs for Gin.
  🔗 [Swaggo GitHub](https://github.com/swaggo/swag)

---

### 🗄️ Database

- **MongoDB (MongoDB Go Driver v2)** — main database, supports geospatial queries with `2dsphere` indexes.
  🔗 [MongoDB Go Driver Docs](https://www.mongodb.com/docs/drivers/go/current/)
  🔗 [MongoDB GitHub](https://github.com/mongodb/mongo-go-driver)
  🔗 [MongoDB Geospatial Queries](https://www.mongodb.com/docs/manual/geospatial-queries/)

---

### ⚡ Caching & Rate Limiting

- **Redis** — caching, sessions, and rate limiting.
  🔗 [go-redis](https://github.com/redis/go-redis)
  🔗 [redis_rate](https://github.com/go-redis/redis_rate)

---

### 📩 Messaging

- **Kafka (segmentio/kafka-go)** — message broker for async communication.
  🔗 [kafka-go GitHub](https://github.com/segmentio/kafka-go)
  🔗 [Introduction to Kafka](https://kafka.apache.org/intro)

---

### 🔑 Security

- **JWT (golang-jwt/jwt/v5)** — authentication and access tokens.
  🔗 [golang-jwt GitHub](https://github.com/golang-jwt/jwt)
- **OAuth2 (golang.org/x/oauth2)** — integration with external providers (Google, GitHub, etc.).
  🔗 [golang.org/x/oauth2 Docs](https://pkg.go.dev/golang.org/x/oauth2)

---

### 📊 Monitoring

- **Prometheus** — metrics collection.
  🔗 [Prometheus Go Client](https://github.com/prometheus/client_golang)
- **Grafana** — visualization dashboard.
  🔗 [Grafana Docs](https://grafana.com/docs/)

---

### 🔌 Real-Time Communication

- **WebSocket** — user ↔ master chat and notifications.
  🔗 [nhooyr/websocket](https://github.com/nhooyr/websocket)
  or
  🔗 [coder/websocket](https://github.com/coder/websocket)
  or
  🔗 [gorilla/websocket](https://github.com/gorilla/websocket)
  Still considering

---

### ⚙️ Dev Environment

- **Docker** — containerization.
  🔗 [Docker Docs](https://docs.docker.com/)
- **Kubernetes** — orchestration.
  🔗 [Kubernetes Docs](https://kubernetes.io/docs/)
- **Tilt** — dev-cycle automation (build + deploy + sync).
  🔗 [Tilt Docs](https://docs.tilt.dev/)
- **Makefile** — standardized commands.
  🔗 [GNU Make Docs](https://www.gnu.org/software/make/manual/make.html)
- **.env config** — configuration management (using envconfig or viper).
  🔗 [envconfig GitHub](https://github.com/kelseyhightower/envconfig)
  🔗 [Viper GitHub](https://github.com/spf13/viper)

---

and maybe payment system Stripe/Lava/Freedom. Still considering
