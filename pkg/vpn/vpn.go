package vpn

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const configPath = "/tmp/vpn/servipar.ovpn"

// InitVPN decodes the base64 profile and writes it to disk
func InitVPN(profileB64 string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := base64.StdEncoding.DecodeString(profileB64)
	if err != nil {
		return fmt.Errorf("Error decoding base64: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	log.Println("=== OpenVPN profile decoded and initialized ===")
	return nil
}

// StartSystemServices initializes tun and dbus
func StartSystemServices() {
	if _, err := os.Stat("/dev/net/tun"); os.IsNotExist(err) {
		exec.Command("sudo", "mkdir", "-p", "/dev/net").Run()
		exec.Command("sudo", "mknod", "/dev/net/tun", "c", "10", "200").Run()
	}

	exec.Command("sudo", "mkdir", "-p", "/run/dbus").Run()
	exec.Command("sudo", "rm", "-f", "/run/dbus/pid").Run()
	exec.Command("sudo", "dbus-uuidgen", "--ensure").Run()

	cmd := exec.Command("sudo", "dbus-daemon", "--system", "--fork")
	if err := cmd.Run(); err != nil {
		log.Println("Warning: dbus-daemon returned error:", err)
	} else {
		log.Println("DBUS daemon started successfully.")
	}
}

// IsTunActive checks if tun0 exists
func IsTunActive() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, i := range interfaces {
		if i.Name == "tun0" {
			return true
		}
	}
	return false
}

// StartSession initiates the openvpn connection and checks for SSO
// Returns (status string, url string, logs string, err error)
func StartSession() (string, string, string, error) {
	log.Println("Starting OpenVPN 3 session...")

	cmd := exec.Command("sudo", "openvpn3", "session-start", "--config", configPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run()

	output := out.String()
	log.Println("openvpn3 output:", output)

	go func() {
		for !IsTunActive() {
			time.Sleep(1 * time.Second)
		}
		log.Println("tun0 detected, attaching log viewer...")
		logCmd := exec.Command("sudo", "openvpn3", "log", "--config", configPath)
		logCmd.Stdout = os.Stdout
		logCmd.Stderr = os.Stderr
		logCmd.Run()
	}()

	re := regexp.MustCompile(`https://[^\s]+`)
	matches := re.FindStringSubmatch(output)

	if len(matches) > 0 {
		url := strings.TrimSpace(matches[0])
		log.Println("ATTENTION: SSO required. Please authorize via the webapp or access:")
		log.Println(url)
		return "auth_required", url, "", nil
	}

	if IsTunActive() {
		return "connected", "", "", nil
	}

	return "error", "", output, fmt.Errorf("No authentication URL or tun0 found")
}

// StopSession disconnects the VPN
func StopSession() {
	log.Println("Disconnecting OpenVPN 3 session...")
	cmd := exec.Command("sudo", "openvpn3", "session-manage", "--disconnect", "--config", configPath)
	cmd.Run()
	log.Println("Session disconnected.")
}
