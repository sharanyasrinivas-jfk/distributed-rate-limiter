# Stage 1: build
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# Stage 2: minimal runtime image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/gateway /usr/local/bin/gateway
COPY configs/config.yaml /etc/gateway/config.yaml
ENV CONFIG_PATH=/etc/gateway/config.yaml
EXPOSE 8080
ENTRYPOINT ["gateway"]
