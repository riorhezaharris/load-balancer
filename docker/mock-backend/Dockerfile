FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mock-backend .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/mock-backend .
EXPOSE 3000
CMD ["./mock-backend"]
