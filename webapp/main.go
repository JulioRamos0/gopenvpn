package main

import (
	"bytes"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var f embed.FS

const configPath = "/tmp/vpn/servipar.ovpn"
const proxiesFilePath = "/data/proxies.json"

type ProxyRule struct {
	Port   int    `json:"port"`
	Target string `json:"target"`
}

var (
	proxiesMu      sync.Mutex
	activeProxies  = make(map[int]*http.Server)
	proxyRulesList []ProxyRule
)

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

	cmd := exec.Command("sudo", "openvpn3", "session-start", "--config", configPath)
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
	cmd := exec.Command("sudo", "openvpn3", "session-manage", "--disconnect", "--config", configPath)
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

// Proxy Management
func saveProxies() error {
	os.MkdirAll(filepath.Dir(proxiesFilePath), 0755)
	data, err := json.Marshal(proxyRulesList)
	if err != nil {
		return err
	}
	return os.WriteFile(proxiesFilePath, data, 0644)
}

func loadProxies() {
	proxiesMu.Lock()
	defer proxiesMu.Unlock()

	data, err := os.ReadFile(proxiesFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println("Error reading proxies file:", err)
		}
		return
	}

	if err := json.Unmarshal(data, &proxyRulesList); err != nil {
		log.Println("Error parsing proxies file:", err)
		return
	}

	for _, rule := range proxyRulesList {
		go startProxyServer(rule)
	}
}

func startProxyServer(rule ProxyRule) error {
	targetURL, err := url.Parse(rule.Target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Header.Set("X-Original-Host", req.Host)
		req.Host = targetURL.Host

		// Prevent leaking Docker's internal network IP to the backend
		req.Header.Del("X-Real-IP")
		req.Header["X-Forwarded-For"] = nil

		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		req.Header.Set("X-Forwarded-Proto", scheme)
	}

	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		originalHost := resp.Request.Header.Get("X-Original-Host")
		if originalHost == "" {
			return nil
		}

		if loc := resp.Header.Get("Location"); loc != "" {
			locURL, err := url.Parse(loc)
			if err == nil && locURL.Host != "" && strings.Contains(locURL.Host, targetURL.Hostname()) {
				locURL.Host = originalHost
				locURL.Scheme = "http"
				resp.Header.Set("Location", locURL.String())
			}
		}

		cookies := resp.Header.Values("Set-Cookie")
		if len(cookies) > 0 {
			resp.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				parts := strings.Split(cookie, ";")
				var newParts []string
				for _, p := range parts {
					if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), "domain=") {
						newParts = append(newParts, p)
					}
				}
				resp.Header.Add("Set-Cookie", strings.Join(newParts, ";"))
			}
		}
		return nil
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", rule.Port),
		Handler: proxy,
	}

	proxiesMu.Lock()
	if existing, exists := activeProxies[rule.Port]; exists {
		existing.Close()
	}
	activeProxies[rule.Port] = server
	proxiesMu.Unlock()

	log.Printf("Starting proxy on port %d -> %s\n", rule.Port, rule.Target)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("Proxy server on port %d failed: %v\n", rule.Port, err)
	}
	return nil
}

func getProxiesHandler(w http.ResponseWriter, _ *http.Request) {
	proxiesMu.Lock()
	defer proxiesMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxyRulesList)
}

func addProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if rule.Port < 1024 || rule.Target == "" {
		http.Error(w, "Invalid port (must be >= 1024) or missing target", http.StatusBadRequest)
		return
	}

	proxiesMu.Lock()
	var existingRuleIndex = -1
	for i, r := range proxyRulesList {
		if r.Port == rule.Port {
			existingRuleIndex = i
			break
		}
	}

	if existingRuleIndex != -1 {
		// Actualizar existente
		proxyRulesList[existingRuleIndex].Target = rule.Target
	} else {
		// Crear nuevo
		proxyRulesList = append(proxyRulesList, rule)
	}

	if err := saveProxies(); err != nil {
		log.Println("Error saving proxies:", err)
	}
	proxiesMu.Unlock()

	go startProxyServer(rule)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rule)
}

func deleteProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	portStr := r.URL.Query().Get("port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	proxiesMu.Lock()
	defer proxiesMu.Unlock()

	found := false
	var newList []ProxyRule
	for _, rule := range proxyRulesList {
		if rule.Port == port {
			found = true
		} else {
			newList = append(newList, rule)
		}
	}

	if !found {
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	proxyRulesList = newList
	if err := saveProxies(); err != nil {
		log.Println("Error saving proxies:", err)
	}

	// Stop server
	if server, exists := activeProxies[port]; exists {
		go server.Close()
		delete(activeProxies, port)
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	loadProxies()
	startSystemServices()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if p, err := strconv.Atoi(port); err == nil && p < 1024 {
		log.Fatalf("Invalid PORT %s: cannot use privileged ports (< 1024) for security reasons", port)
	}

	http.HandleFunc("/api/start", startHandler)
	http.HandleFunc("/api/stop", stopHandler)
	http.HandleFunc("/api/status", statusHandler)

	http.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getProxiesHandler(w, r)
		case http.MethodPost:
			addProxyHandler(w, r)
		case http.MethodDelete:
			deleteProxyHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

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
