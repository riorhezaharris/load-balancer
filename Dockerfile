FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o load-balancer .

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/load-balancer .
EXPOSE 8080 9090
ENTRYPOINT ["./load-balancer"]
