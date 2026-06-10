FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/blackflow ./cmd/blackflow

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/bin/blackflow .

EXPOSE 80
CMD ["./blackflow"]
