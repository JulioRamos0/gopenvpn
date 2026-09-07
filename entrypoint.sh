#!/bin/bash
set -e

if [ -z "$OPENVPN_PROFILE" ]; then
    echo "[ERROR] OPENVPN_PROFILE in Base64 is missing."
    exit 1
fi

if [ ! -c /dev/net/tun ]; then
    mkdir -p /dev/net
    mknod /dev/net/tun c 10 200
fi

rm -f /var/run/dbus/pid
dbus-daemon --system --fork

mkdir -p /tmp/vpn
echo "$OPENVPN_PROFILE" | base64 -d > /tmp/vpn/servipar.ovpn
echo "=== OpenVPN profile decoded ==="

echo "Starting OpenVPN 3 session..."
echo "ATTENTION: If SSO is required, look for the URL below, copy and paste it into your browser."

openvpn3 session-start --config /tmp/vpn/servipar.ovpn

echo "Waiting for you to complete SSO authentication in your browser..."
echo "The script will pause here until the tun0 interface is created by the VPN."

while ! ip link show tun0 > /dev/null 2>&1; do
    sleep 2
done

echo "✅ tun0 interface detected. The VPN is connected."

echo "Keeping the container alive and showing logs..."
exec openvpn3 log --config /tmp/vpn/servipar.ovpn
