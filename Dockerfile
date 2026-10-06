# Mangle VPN in one container: the web application, its background work and
# OpenVPN, run by "mangle-vpn run". See "Running in Docker" in the README.

# The web interface and the binary are built once, on the building
# machine's own platform, and the binary cross-compiled for each platform
# the image is for; only the last stage is assembled per platform.

# The web interface, compiled into the binary.
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY ui/ ./
RUN mkdir -p ../internal/webui/dist && npm run build

# The binary, statically linked, for the platform the image is for.
FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/mangle-vpn ./cmd/mangle

# OpenVPN 2.6 and iptables from Debian, and tini to reap the processes
# OpenVPN's hooks leave behind.
FROM debian:trixie-slim

LABEL org.opencontainers.image.title="Mangle VPN" \
      org.opencontainers.image.description="A self-hosted OpenVPN server with a web interface for managing who can connect and what they can reach." \
      org.opencontainers.image.source="https://github.com/jeffmvr/mangle-vpn" \
      org.opencontainers.image.licenses="GPL-3.0-only"

RUN apt-get update \
 && apt-get install -y --no-install-recommends openvpn iptables tini ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/mangle-vpn /opt/mangle-vpn/mangle-vpn
WORKDIR /opt/mangle-vpn

# Everything worth keeping: the database, keys, logs and backups.
VOLUME /opt/mangle-vpn/data

EXPOSE 80/tcp 443/tcp 1194/udp

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s \
  CMD ["/opt/mangle-vpn/mangle-vpn", "health"]

ENTRYPOINT ["/usr/bin/tini", "--", "/opt/mangle-vpn/mangle-vpn"]
CMD ["run"]
