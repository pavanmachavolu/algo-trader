FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Tidy dependencies and build the application
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /app/backend ./main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/backend .

EXPOSE 8080

CMD ["./backend"]
