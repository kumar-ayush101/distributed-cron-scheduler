# STAGE 1: Build the binary
FROM golang:alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN go build -o main ./cmd/server

# STAGE 2: Run the binary (Tiny Image)
FROM alpine:latest

WORKDIR /root/

# Copy only the compiled binary from Stage 1
COPY --from=builder /app/main .

# Command to run the app
CMD ["./main"]