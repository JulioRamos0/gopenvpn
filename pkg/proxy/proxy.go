package proxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const proxiesFilePath = "/data/proxies.json"

type ProxyRule struct {
	Port   int    `json:"port"`
	Target string `json:"target"`
	Status string `json:"status,omitempty"`
}

var (
	proxiesMu      sync.Mutex
	activeProxies  = make(map[int]*http.Server)
	proxyRulesList []ProxyRule
)

// SaveProxies persists the proxy rules to a JSON file
func SaveProxies() error {
	os.MkdirAll(filepath.Dir(proxiesFilePath), 0755)
	data, err := json.Marshal(proxyRulesList)
	if err != nil {
		return err
	}
	return os.WriteFile(proxiesFilePath, data, 0644)
}

// LoadProxies reads the JSON file and starts the proxy servers
func LoadProxies() {
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
		go StartProxyServer(rule)
	}
}

// StartProxyServer starts a single HTTP proxy server based on a ProxyRule
func StartProxyServer(rule ProxyRule) error {
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

// GetProxies returns a copy of the current proxy rules
func GetProxies() []ProxyRule {
	proxiesMu.Lock()
	defer proxiesMu.Unlock()
	
	cpy := make([]ProxyRule, len(proxyRulesList))
	for i, r := range proxyRulesList {
		if _, exists := activeProxies[r.Port]; exists {
			r.Status = "active"
		} else {
			r.Status = "stopped"
		}
		cpy[i] = r
	}
	return cpy
}

// StopAllProxies closes all active proxy servers
func StopAllProxies() {
	proxiesMu.Lock()
	defer proxiesMu.Unlock()
	for port, server := range activeProxies {
		go server.Close()
		delete(activeProxies, port)
	}
}

// StartAllProxies starts all configured proxy servers that are not already running
func StartAllProxies() {
	proxiesMu.Lock()
	var toStart []ProxyRule
	for _, rule := range proxyRulesList {
		if _, exists := activeProxies[rule.Port]; !exists {
			toStart = append(toStart, rule)
		}
	}
	proxiesMu.Unlock()

	for _, rule := range toStart {
		go StartProxyServer(rule)
	}
}

// AddProxy adds or updates a proxy rule and starts it
func AddProxy(rule ProxyRule) error {
	proxiesMu.Lock()
	
	var existingRuleIndex = -1
	for i, r := range proxyRulesList {
		if r.Port == rule.Port {
			existingRuleIndex = i
			break
		}
	}

	if existingRuleIndex != -1 {
		proxyRulesList[existingRuleIndex].Target = rule.Target
	} else {
		proxyRulesList = append(proxyRulesList, rule)
	}

	if err := SaveProxies(); err != nil {
		log.Println("Error saving proxies:", err)
	}
	proxiesMu.Unlock()

	go StartProxyServer(rule)
	return nil
}

// DeleteProxy removes a proxy rule and stops the server
func DeleteProxy(port int) error {
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
		return fmt.Errorf("Proxy not found")
	}

	proxyRulesList = newList
	if err := SaveProxies(); err != nil {
		log.Println("Error saving proxies:", err)
	}

	// Stop server
	if server, exists := activeProxies[port]; exists {
		go server.Close()
		delete(activeProxies, port)
	}

	return nil
}
