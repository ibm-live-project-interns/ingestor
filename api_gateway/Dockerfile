FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy shared module first
COPY ingestor/shared/ ./ingestor/shared/

# Copy api_gateway files
COPY ingestor/api_gateway/go.mod ./ingestor/api_gateway/
WORKDIR /app/ingestor/api_gateway
RUN go mod download || true

COPY ingestor/api_gateway/ .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o api_gateway .

FROM alpine:latest
RUN apk --no-cache add ca-certificates docker-cli
WORKDIR /root/
COPY --from=builder /app/ingestor/api_gateway/api_gateway .
COPY --from=builder /app/ingestor/api_gateway/templates ./templates/

EXPOSE 8080
CMD ["./api_gateway"]
