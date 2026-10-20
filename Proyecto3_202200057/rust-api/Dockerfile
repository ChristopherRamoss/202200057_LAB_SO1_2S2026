# Etapa 1: compilar
FROM rust:1.90 AS builder
WORKDIR /app
ENV CARGO_BUILD_JOBS=2

# Capa de dependencias: solo cambia si cambia Cargo.toml o Cargo.lock
COPY Cargo.toml Cargo.lock* ./
RUN mkdir src && echo "fn main() {}" > src/main.rs \
    && cargo build --release \
    && rm -rf src

# Capa del código: la que cambia seguido
COPY src ./src
RUN touch src/main.rs && cargo build --release

# Etapa 2: imagen final, pequeña
FROM debian:bookworm-slim
COPY --from=builder /app/target/release/rust-api /usr/local/bin/rust-api
EXPOSE 8000
CMD ["rust-api"]
