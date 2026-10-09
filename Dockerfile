# -- Stage 1: Build Go Webapp --
FROM golang:1.26-bookworm AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /gopenvpn ./cmd/server/main.go

# -- Stage 2: Final Image --
FROM debian:bookworm

ENV DEBIAN_FRONTEND=noninteractive
ENV PORT=8080

RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    openssh-client \
    gnupg \
    ca-certificates \
    apt-transport-https \
    iproute2 \
    dbus \
    sudo \
    && mkdir -p /etc/apt/keyrings \
    && curl -fsSL https://packages.openvpn.net/packages-repo.gpg > /etc/apt/keyrings/openvpn.asc \
    && echo "deb [signed-by=/etc/apt/keyrings/openvpn.asc] https://packages.openvpn.net/openvpn3/debian bookworm main" > /etc/apt/sources.list.d/openvpn3.list \
    && apt-get update \
    && apt-get install -y openvpn3 \
    && curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf awscliv2.zip aws \
    && curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o "session-manager-plugin.deb" \
    && dpkg -i session-manager-plugin.deb \
    && rm session-manager-plugin.deb \
    && apt-get remove -y gnupg apt-transport-https unzip \
    && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -r appgroup && useradd -r -g appgroup appuser

RUN echo "appuser ALL=(ALL) NOPASSWD: /usr/bin/openvpn3, /usr/bin/dbus-daemon, /usr/bin/dbus-uuidgen, /usr/bin/mknod, /usr/bin/mkdir, /usr/bin/rm" > /etc/sudoers.d/appuser \
    && chmod 0440 /etc/sudoers.d/appuser

# Copy the Go binary from builder
COPY --from=builder /gopenvpn /usr/local/bin/gopenvpn
RUN chmod +x /usr/local/bin/gopenvpn

RUN mkdir -p /data /tmp/vpn /run/dbus \
    && chown -R appuser:appgroup /data /tmp/vpn /run/dbus

EXPOSE $PORT

ENTRYPOINT ["/usr/local/bin/gopenvpn"]
