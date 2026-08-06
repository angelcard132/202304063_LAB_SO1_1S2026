FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o api2 main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api2 .
EXPOSE 8082
CMD ["./api2"]
