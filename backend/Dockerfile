# --- Stage 1: build the Go binary from source ---
FROM golang:1.26-bookworm AS build
WORKDIR /src
# Cache deps first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# --- Stage 2: runtime with a pg_dump matching docker-compose PostgreSQL 16 ---
FROM postgres:16-alpine
# ca-certificates: outbound HTTPS to the AI providers. tzdata: POSTGRES_DSN sets
# TimeZone=Asia/Shanghai. wget: container healthcheck.
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/api /app/api
# Local fallback for generated media / reference uploads (RustFS/S3 is primary).
RUN mkdir -p /app/data/generated /app/data/backups && chmod +x /app/api
ENV HTTP_ADDR=0.0.0.0:6666
EXPOSE 6666
ENTRYPOINT ["/app/api"]
