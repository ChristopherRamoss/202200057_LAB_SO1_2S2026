FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY proto ./proto
COPY go-deployment2/grpc-server ./go-deployment2/grpc-server
RUN CGO_ENABLED=0 go build -o /out/app ./go-deployment2/grpc-server

FROM alpine:3.20
COPY --from=builder /out/app /usr/local/bin/app
CMD ["app"]
