# Etapa 1: compilar
FROM rust:1.90 AS builder
WORKDIR /app
COPY . .
ENV CARGO_BUILD_JOBS=2
RUN cargo build --release

# Etapa 2: imagen final, mucho más pequeña
FROM debian:bookworm-slim
COPY --from=builder /app/target/release/rust-api /usr/local/bin/rust-api
EXPOSE 8000
CMD ["rust-api"]