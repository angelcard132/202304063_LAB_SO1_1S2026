# Build stage
FROM rust:slim as builder

WORKDIR /usr/src/app
COPY Cargo.toml Cargo.lock ./
# Precompilar dependencias vacías para cache
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release && rm -rf src

COPY src ./src
RUN touch src/main.rs && cargo build --release

# Runtime stage
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /usr/src/app/target/release/api_rust /app/api_rust

ENV PORT=8080
EXPOSE 8080

CMD ["/app/api_rust"]
