# PRD 13.1: Docker build. Multi-stage for smaller image.
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o qcs-server ./cmd/server
RUN go build -o qcs-migrate ./cmd/migrate

FROM alpine:3.19
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=builder /app/qcs-server /app/qcs-migrate ./
COPY --from=builder /app/web /app/web
COPY --from=builder /app/sql/migrations /app/sql/migrations
ENV PORT=8080
WORKDIR /app
EXPOSE 8080
CMD ["./qcs-server"]
