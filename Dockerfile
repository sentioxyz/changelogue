# Stage 1: Build Next.js static export
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/out ./web/out
RUN CGO_ENABLED=0 go build -o changelogue ./cmd/server

# Stage 3: Minimal runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/changelogue .
COPY --from=backend /app/web/out ./web/out
EXPOSE 8080
CMD ["./changelogue"]
