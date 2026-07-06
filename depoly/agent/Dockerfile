# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24-alpine

FROM --platform=$TARGETPLATFORM golang:${GO_VERSION} AS go-builder
WORKDIR /src

ARG VERSION=dev

ENV CGO_ENABLED=1

RUN apk add --no-cache build-base ca-certificates

COPY . .

RUN go mod download
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/rivo-agent ./cmd/agent

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata iputils \
    && addgroup -S rivo \
    && adduser -S -D -H -h /app -s /sbin/nologin -G rivo -u 10001 rivo

WORKDIR /app

COPY --from=go-builder /out/rivo-agent /app/rivo-agent
RUN mkdir -p /app/data /app/logs \
    && chown -R rivo:rivo /app

VOLUME ["/app/data", "/app/logs"]

ENTRYPOINT ["/app/rivo-agent"]
