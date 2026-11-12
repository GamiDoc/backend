FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o gamidoc-backend ./cmd/gamidoc-backend/

FROM alpine:3.21
WORKDIR /app
# Chromium prints the design documents (HTML templates) to PDF without a separate
# Gotenberg service; the fonts give it Latin text, symbols and arrows.
RUN apk add --no-cache ca-certificates tzdata chromium font-liberation font-dejavu
COPY --from=builder /app/gamidoc-backend .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/rule ./rule
EXPOSE 8080
CMD ["sh", "-c", "./gamidoc-backend migrate up && ./gamidoc-backend serve"]
