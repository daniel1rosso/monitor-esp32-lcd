# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM base AS development
ARG AIR_VERSION=v1.67.4
RUN go install github.com/air-verse/air@${AIR_VERSION}
COPY . .
CMD ["air", "-c", ".air.toml"]

FROM base AS build
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILT_AT=unknown
RUN CGO_ENABLED=0 go test -tags=nomsgpack ./... && \
    CGO_ENABLED=0 go build -tags=nomsgpack -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILT_AT}" \
      -o /out/desk-monitor ./cmd/desk-monitor

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata mosquitto mosquitto-clients && \
    addgroup -S -g 10001 desk && \
    adduser -S -D -H -u 10001 -G desk desk && \
    mkdir -p /data /config && chown -R desk:desk /data /config
COPY --from=build /out/desk-monitor /usr/local/bin/desk-monitor
USER desk
EXPOSE 8080 9090
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/desk-monitor"]
CMD ["server"]
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
  CMD ["/usr/local/bin/desk-monitor", "probe"]
