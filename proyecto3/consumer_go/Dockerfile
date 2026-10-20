# Multi-stage Dockerfile for Go Consumer
FROM golang:alpine AS builder

WORKDIR /app
COPY consumer_go /app

RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o consumer_go .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates curl
WORKDIR /app
COPY --from=builder /app/consumer_go /app/consumer_go

CMD ["/app/consumer_go"]
