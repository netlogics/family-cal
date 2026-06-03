# ── Builder ───────────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine AS builder

WORKDIR /app

# Tailwind's musl binary needs C++ runtime libraries
RUN apk add --no-cache libstdc++ libgcc

# Install Tailwind CLI (same version used in development, arch-aware)
ARG TAILWIND_VERSION=4.3.0
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then TAILWIND_ARCH="x64-musl"; \
    elif [ "$ARCH" = "aarch64" ]; then TAILWIND_ARCH="arm64-musl"; \
    else TAILWIND_ARCH="x64-musl"; fi && \
    wget -qO /usr/local/bin/tailwindcss \
      "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-linux-${TAILWIND_ARCH}" \
    && chmod +x /usr/local/bin/tailwindcss

# Cache Go modules before copying source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN tailwindcss -i cmd/server/web/input.css \
                -o cmd/server/web/static/output.css \
                --minify \
    && CGO_ENABLED=0 go build -ldflags="-s -w" -o family-cal ./cmd/server

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/family-cal .

VOLUME ["/data"]
ENV DB_PATH=/data/family-cal.db

EXPOSE 8080
CMD ["./family-cal"]
