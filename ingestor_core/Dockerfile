# Build from ingestor/ directory to include shared package
# docker build -f ingestor_core/Dockerfile -t ingestor-core .
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy shared package first
COPY shared/ ./shared/

# Copy ingestor_core
COPY ingestor_core/ ./ingestor_core/

# Build
WORKDIR /app/ingestor_core
RUN go mod download
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o ingestor_core main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/ingestor_core/ingestor_core .

EXPOSE 8001
CMD ["./ingestor_core"]
