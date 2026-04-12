# UI build
FROM node:20-alpine AS ui
WORKDIR /ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# Go build
FROM golang:1.25-alpine AS api
WORKDIR /src
RUN apk add --no-cache git
COPY go.* ./
RUN go mod download
COPY . .
COPY --from=ui /ui/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /observer ./cmd/observer

# Runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=api /observer /observer
COPY --from=api /src/migrations /migrations
EXPOSE 8080
ENV MIGRATIONS_DIR=/migrations
USER nonroot:nonroot
ENTRYPOINT ["/observer"]
