# Build from ingestor/ directory to include shared package
# docker build -f event_router/Dockerfile -t event-router .
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy shared package first
COPY shared/ ./shared/

# Copy event_router
COPY event_router/ ./event_router/

# Build
WORKDIR /app/event_router
RUN go mod download
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o event_router main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/event_router/event_router .
COPY --from=builder /app/event_router/config.json .

EXPOSE 8082
CMD ["./event_router"]
