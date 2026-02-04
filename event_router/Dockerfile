# Build from ingestor/ directory to include shared package
# docker build -f event_router/Dockerfile -t event-router .
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy shared package first
COPY shared/ ./shared/

# Copy event_router
COPY event_router/ ./event_router/

# Build
WORKDIR /app/event_router
RUN go mod download
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux go build -o event_router .

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /root/
COPY --from=builder /app/event_router/event_router .
COPY --from=builder /app/event_router/config.json .

EXPOSE 8082
CMD ["./event_router"]
