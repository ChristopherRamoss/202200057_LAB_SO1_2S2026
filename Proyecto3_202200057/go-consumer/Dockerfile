# Etapa 1: compilar
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY proto ./proto
COPY go-consumer ./go-consumer
RUN CGO_ENABLED=0 go build -o /out/consumer ./go-consumer

# Etapa 2: imagen final, pequeña
FROM alpine:3.20
COPY --from=builder /out/consumer /usr/local/bin/consumer
CMD ["consumer"]
