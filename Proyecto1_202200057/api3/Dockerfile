FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go build -o api3 main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api3 .
EXPOSE 8083
CMD ["./api3"]