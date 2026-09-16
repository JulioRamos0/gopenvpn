package api

import (
	"encoding/json"
	"net/http"

	"gopenvpn/config"
	"gopenvpn/pkg/proxy"
	"gopenvpn/pkg/tunnel"
	"gopenvpn/pkg/vpn"
)

func StartHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := vpn.InitVPN(cfg.OpenVPNProfile); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		status, url, logs, err := vpn.StartSession()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "logs": logs})
			return
		}

		if status == "auth_required" {
			json.NewEncoder(w).Encode(map[string]interface{}{"url": url, "status": "auth_required"})
			return
		}

		proxy.StartAllProxies()

		json.NewEncoder(w).Encode(map[string]interface{}{"status": "connected"})
	}
}

func StopHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vpn.StopSession()
		proxy.StopAllProxies()
		tunnel.StopAllTunnels()

		json.NewEncoder(w).Encode(map[string]interface{}{"status": "disconnected"})
	}
}

func StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		connected := vpn.IsTunActive()
		json.NewEncoder(w).Encode(map[string]interface{}{"connected": connected})
	}
}
