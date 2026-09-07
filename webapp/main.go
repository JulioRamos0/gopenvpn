package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed index.html
var f embed.FS

const configPath = "/tmp/vpn/servipar.ovpn"

func initVPN() error {
	profileB64 := os.Getenv("OPENVPN_PROFILE")
	if profileB64 == "" {
		return fmt.Errorf("OPENVPN_PROFILE environment variable is not defined")
	}

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

func startSystemServices() {
	if _, err := os.Stat("/dev/net/tun"); os.IsNotExist(err) {
		os.MkdirAll("/dev/net", 0755)
		exec.Command("mknod", "/dev/net/tun", "c", "10", "200").Run()
	}

	exec.Command("mkdir", "-p", "/run/dbus").Run()
	exec.Command("rm", "-f", "/run/dbus/pid").Run()
	exec.Command("dbus-uuidgen", "--ensure").Run()

	cmd := exec.Command("dbus-daemon", "--system", "--fork")
	if err := cmd.Run(); err != nil {
		log.Println("Warning: dbus-daemon returned error:", err)
	} else {
		log.Println("DBUS daemon started successfully.")
	}
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := initVPN(); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	log.Println("Starting OpenVPN 3 session...")

	cmd := exec.Command("openvpn3", "session-start", "--config", configPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run()

	output := out.String()
	log.Println("openvpn3 output:", output)

	go func() {
		for !isTunActive() {
			time.Sleep(1 * time.Second)
		}
		log.Println("tun0 detected, attaching log viewer...")
		logCmd := exec.Command("openvpn3", "log", "--config", configPath)
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

		json.NewEncoder(w).Encode(map[string]interface{}{"url": url, "status": "auth_required"})
		return
	}

	if isTunActive() {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "connected"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"error": "No authentication URL or tun0 found", "logs": output})
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Disconnecting OpenVPN 3 session...")
	cmd := exec.Command("openvpn3", "session-manage", "--disconnect", "--config", configPath)
	cmd.Run()
	log.Println("Session disconnected.")

	json.NewEncoder(w).Encode(map[string]interface{}{"status": "disconnected"})
}

func isTunActive() bool {
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

func statusHandler(w http.ResponseWriter, r *http.Request) {
	connected := isTunActive()
	json.NewEncoder(w).Encode(map[string]interface{}{"connected": connected})
}

func main() {
	startSystemServices()

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	http.HandleFunc("/api/start", startHandler)
	http.HandleFunc("/api/stop", stopHandler)
	http.HandleFunc("/api/status", statusHandler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFS(f, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t.Execute(w, nil)
	})

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
