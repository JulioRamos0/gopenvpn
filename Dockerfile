# -- Stage 1: Build Go Webapp --
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY webapp/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /openvpn-webapp main.go

# -- Stage 2: Final Image --
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PORT=80

RUN apt-get update && apt-get install -y \
    curl \
    gnupg \
    ca-certificates \
    apt-transport-https \
    iproute2 \
    dbus \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://swupdate.openvpn.net/repos/openvpn-repo-pkg-key.pub | gpg --dearmor > /etc/apt/keyrings/openvpn.gpg \
    && echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/openvpn.gpg] https://swupdate.openvpn.net/community/openvpn3/repos jammy main" > /etc/apt/sources.list.d/openvpn3.list \
    && apt-get update \
    && apt-get install -y openvpn3 \
    && apt-get remove -y curl gnupg apt-transport-https \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/run/dbus

# Copy the Go binary from builder
COPY --from=builder /openvpn-webapp /usr/local/bin/openvpn-webapp
RUN chmod +x /usr/local/bin/openvpn-webapp

EXPOSE $PORT

ENTRYPOINT ["/usr/local/bin/openvpn-webapp"]
