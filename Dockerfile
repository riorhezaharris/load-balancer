FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o load-balancer .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/load-balancer .
EXPOSE 8080 9090
ENTRYPOINT ["./load-balancer"]
