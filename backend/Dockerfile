# Build stage.
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cuanku-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cuanku-migration ./cmd/migration

# Runtime stage.
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 cuanku

USER cuanku
WORKDIR /app

COPY --from=build /out/cuanku-api /app/cuanku-api
COPY --from=build /out/cuanku-migration /app/cuanku-migration

EXPOSE 8080

# The DSN is injected at runtime through DATABASE_URL; nothing is baked in.
ENTRYPOINT ["/app/cuanku-api"]
