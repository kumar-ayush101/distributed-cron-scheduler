# syntax=docker/dockerfile:1

# ---------- Stage 1: build the React frontend ----------
FROM node:22-alpine AS frontend
WORKDIR /app/client
COPY client/package.json client/package-lock.json ./
RUN npm ci
COPY client/ ./
# Build with an empty API base so the SPA calls the same origin that serves it.
ENV VITE_API_URL=""
RUN npm run build
# Output is emitted to /app/web (see client/vite.config.js build.outDir).

# ---------- Stage 2: build the Go binary ----------
FROM golang:1.25-alpine AS backend
# Pin the toolchain so the module's go directive does not trigger an auto-upgrade.
ENV GOTOOLCHAIN=local
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# ---------- Stage 3: minimal runtime image ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=backend /app/server ./server
COPY --from=frontend /app/web ./web
ENV SERVE_FRONTEND=true \
    FRONTEND_DIR=./web \
    APP_ENV=production \
    PORT=8080
EXPOSE 8080
USER appuser
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -q --spider http://localhost:8080/health || exit 1
ENTRYPOINT ["./server"]
