# -- Stage 1: Build Go Webapp --
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY webapp/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /openvpn-webapp main.go

# -- Stage 2: Final Image --
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PORT=8080

RUN apt-get update && apt-get install -y \
    curl \
    gnupg \
    ca-certificates \
    apt-transport-https \
    iproute2 \
    dbus \
    sudo \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://swupdate.openvpn.net/repos/openvpn-repo-pkg-key.pub | gpg --dearmor > /etc/apt/keyrings/openvpn.gpg \
    && echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/openvpn.gpg] https://swupdate.openvpn.net/community/openvpn3/repos jammy main" > /etc/apt/sources.list.d/openvpn3.list \
    && apt-get update \
    && apt-get install -y openvpn3 \
    && apt-get remove -y curl gnupg apt-transport-https \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -r appgroup && useradd -r -g appgroup appuser

RUN echo "appuser ALL=(ALL) NOPASSWD: /usr/bin/openvpn3, /usr/bin/dbus-daemon, /usr/bin/dbus-uuidgen, /usr/bin/mknod, /usr/bin/mkdir, /usr/bin/rm" > /etc/sudoers.d/appuser \
    && chmod 0440 /etc/sudoers.d/appuser

# Copy the Go binary from builder
COPY --from=builder /openvpn-webapp /usr/local/bin/openvpn-webapp
RUN chmod +x /usr/local/bin/openvpn-webapp

RUN mkdir -p /data /tmp/vpn /run/dbus \
    && chown -R appuser:appgroup /data /tmp/vpn /run/dbus

USER appuser

EXPOSE $PORT

ENTRYPOINT ["/usr/local/bin/openvpn-webapp"]
