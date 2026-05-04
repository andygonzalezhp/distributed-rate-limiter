FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o demo-api ./cmd/demo-api
RUN go build -o gateway ./cmd/gateway

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/demo-api .
COPY --from=builder /app/gateway .

EXPOSE 8080
EXPOSE 8081