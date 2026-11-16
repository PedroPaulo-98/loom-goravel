FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /build

# Dependencies first, so code changes don't re-download modules.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags "-s -w" -o main .

FROM alpine:3.21

# ca-certificates: SMTP/HTTPS with TLS; tzdata: correct local times.
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

WORKDIR /www

COPY --from=builder /build/main /www/main
COPY --from=builder /build/public/ /www/public/
COPY --from=builder /build/resources/ /www/resources/

# Uploads, sessions and logs (mounted as a volume in docker-compose.yml).
RUN mkdir -p storage/app storage/framework/sessions storage/logs \
    && chown -R app:app /www/storage

USER app

# The settings come from environment variables (see docker-compose.yml);
# the .env file is not copied into the image.
ENV APP_HOST=0.0.0.0 \
    APP_PORT=3000

EXPOSE 3000

ENTRYPOINT ["/www/main"]
