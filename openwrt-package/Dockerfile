# OpenWrt build environment for xxk-proxy
# This Dockerfile sets up a build environment for cross-compiling xxk-proxy for OpenWrt

FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

ENV GOPROXY=https://goproxy.cn,direct
ENV GOOS=linux
ENV CGO_ENABLED=0

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/main.go ./
RUN go build -trimpath -ldflags="-s -w" -o xxk-proxy .

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /var/log/proxy /etc/config

COPY --from=builder /src/xxk-proxy /usr/bin/xxk-proxy
COPY files/xxk-proxy.init /etc/init.d/xxk-proxy
COPY files/xxk-proxy.config /etc/config/xxk-proxy
COPY files/xxk-proxy.hotplug /etc/hotplug.d/iface/60-xxk-proxy
COPY files/xxk-proxy.sh /lib/functions/xxk-proxy.sh
COPY config.yaml /etc/config/xxk-proxy.yaml

RUN chmod +x /usr/bin/xxk-proxy /etc/init.d/xxk-proxy /etc/hotplug.d/iface/60-xxk-proxy

EXPOSE 5858

ENTRYPOINT ["/usr/bin/xxk-proxy"]
CMD ["-port=5858", "-log=true", "-logfile=/var/log/proxy/proxy.log"]