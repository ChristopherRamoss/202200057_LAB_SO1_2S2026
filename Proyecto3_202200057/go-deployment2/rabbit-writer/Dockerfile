FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY proto ./proto
COPY go-deployment2/rabbit-writer ./go-deployment2/rabbit-writer
RUN CGO_ENABLED=0 go build -o /out/app ./go-deployment2/rabbit-writer

FROM alpine:3.20
COPY --from=builder /out/app /usr/local/bin/app
CMD ["app"]
