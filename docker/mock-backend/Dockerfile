FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mock-backend .

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/mock-backend .
EXPOSE 3000
CMD ["./mock-backend"]
