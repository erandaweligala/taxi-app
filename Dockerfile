# Multi-stage build producing all four service binaries in one small image.
# docker-compose selects which binary to run per service via `command:`.
FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api            ./cmd/api && \
    CGO_ENABLED=0 go build -o /out/locationworker ./cmd/locationworker && \
    CGO_ENABLED=0 go build -o /out/tripworker     ./cmd/tripworker && \
    CGO_ENABLED=0 go build -o /out/gateway        ./cmd/gateway && \
    CGO_ENABLED=0 go build -o /out/rider          ./apps/rider && \
    CGO_ENABLED=0 go build -o /out/driver         ./apps/driver

FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/ /usr/local/bin/
USER app
# Default command; overridden per service in docker-compose.
CMD ["api"]
