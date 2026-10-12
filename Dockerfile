# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web/embed.go ./web/embed.go
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/canvasbin ./cmd/canvasbin

FROM alpine:3.22
RUN apk add --no-cache ca-certificates wget chromium font-noto font-noto-emoji ttf-freefont \
    && adduser -D -H -u 10001 canvasbin
COPY --from=build /out/canvasbin /usr/local/bin/canvasbin
# Chromium keeps its sandbox. Docker's default seccomp profile blocks the
# namespaces that sandbox needs; the compose files set seccomp=unconfined.
USER canvasbin
ENV HTTP_ADDR=:8080 \
    CHROME_BIN=/usr/bin/chromium-browser \
    HOME=/tmp
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/canvasbin"]
