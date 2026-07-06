# syntax=docker/dockerfile:1

ARG NODE_VERSION=22-alpine
ARG GO_VERSION=1.24-alpine

FROM node:${NODE_VERSION} AS panel-builder
WORKDIR /src/panel
ENV CI=true
RUN corepack enable
COPY panel/package.json panel/pnpm-lock.yaml panel/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY panel/ ./
RUN pnpm build

FROM --platform=$TARGETPLATFORM golang:${GO_VERSION} AS go-builder
WORKDIR /src

ARG VERSION=dev

ENV CGO_ENABLED=1

RUN apk add --no-cache build-base ca-certificates

COPY . .
RUN rm -rf ./internal/master/web/dist/*
COPY --from=panel-builder /src/panel/dist/ ./internal/master/web/dist/

RUN go mod download
RUN go build -trimpath -ldflags "-s -w" -o /out/rivo-master ./cmd/master

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S rivo \
    && adduser -S -D -H -h /app -s /sbin/nologin -G rivo -u 10001 rivo

WORKDIR /app

COPY --from=go-builder /out/rivo-master /app/rivo-master
RUN mkdir -p /app/data /app/logs \
    && chown -R rivo:rivo /app

USER rivo

EXPOSE 8080 9443
VOLUME ["/app/data", "/app/logs"]

ENTRYPOINT ["/app/rivo-master"]
CMD ["-config", "/app/config.yaml"]
