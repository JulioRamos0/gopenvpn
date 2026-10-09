package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"gopenvpn/config"
	"gopenvpn/pkg/tunnel"
)

// TunnelsHandler routes the CRUD and action endpoints for Tunnels.
func TunnelsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Basic routing based on Method and Path
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		
		// Expected paths:
		// GET /api/tunnels
		// POST /api/tunnels
		// PUT /api/tunnels/{id}
		// DELETE /api/tunnels/{id}
		// POST /api/tunnels/{id}/start
		// POST /api/tunnels/{id}/stop

		if len(pathParts) == 2 {
			// /api/tunnels
			switch r.Method {
			case http.MethodGet:
				getTunnelsHandler(w, r)
			case http.MethodPost:
				addTunnelHandler(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		if len(pathParts) >= 3 {
			// /api/tunnels/{id}
			id := pathParts[2]
			
			if len(pathParts) == 3 {
				switch r.Method {
				case http.MethodPut:
					updateTunnelHandler(w, r, id)
				case http.MethodDelete:
					deleteTunnelHandler(w, r, id)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

			if len(pathParts) == 4 {
				// /api/tunnels/{id}/{action}
				action := pathParts[3]
				
				switch action {
				case "start":
					if r.Method != http.MethodPost {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					startTunnelHandler(w, r, id)
				case "stop":
					if r.Method != http.MethodPost {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					stopTunnelHandler(w, r, id)
				case "sso-login":
					if r.Method != http.MethodPost {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					ssoLoginTunnelHandler(w, r, id)
				case "sso-status":
					if r.Method != http.MethodGet {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					ssoStatusTunnelHandler(w, r, id)
				default:
					http.Error(w, "Action not found", http.StatusNotFound)
				}
				return
			}
		}

		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func getTunnelsHandler(w http.ResponseWriter, _ *http.Request) {
	tunnels, err := tunnel.GetTunnels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Attach memory status and SSO validation to each tunnel
	for i, t := range tunnels {
		tunnels[i].Status = tunnel.GetStatus(t.ID)

		if t.Type == tunnel.TypeAWSSSM && t.IsSSO {
			profile := strings.TrimSpace(t.AWSProfile)
			if profile == "" {
				profile = "default"
			}

			// Check if active session in memory
			if sess, exists := tunnel.GetSSOSession(t.ID); exists && !sess.Done {
				tunnels[i].SSOAuthURL = sess.AuthURL
				tunnels[i].SSOUserCode = sess.UserCode
				tunnels[i].Status = "authenticating"
			} else {
				isAuth := tunnel.CheckSSOAuthenticated(profile)
				tunnels[i].SSOAuthenticated = isAuth
				if !isAuth && (tunnels[i].Status == "stopped" || tunnels[i].Status == "") {
					tunnels[i].Status = "needs_login"
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tunnels)
}

func addTunnelHandler(w http.ResponseWriter, r *http.Request) {
	var t tunnel.Tunnel
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if t.ID == "" || t.Name == "" || t.RemoteHost == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Ensure AWS config files are updated for the tunnel
	if t.Type == tunnel.TypeAWSSSM {
		if err := tunnel.EnsureAWSConfig(t); err != nil {
			http.Error(w, "Failed to configure AWS settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tunnel.AddTunnel(t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func updateTunnelHandler(w http.ResponseWriter, r *http.Request, id string) {
	var t tunnel.Tunnel
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if t.Type == tunnel.TypeAWSSSM {
		if err := tunnel.EnsureAWSConfig(t); err != nil {
			http.Error(w, "Failed to configure AWS settings: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tunnel.UpdateTunnel(id, t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteTunnelHandler(w http.ResponseWriter, _ *http.Request, id string) {
	if err := tunnel.DeleteTunnel(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func startTunnelHandler(w http.ResponseWriter, _ *http.Request, id string) {
	tunnels, err := tunnel.GetTunnels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, t := range tunnels {
		if t.ID == id {
			if err := tunnel.StartTunnel(t); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "Tunnel not found", http.StatusNotFound)
}

func stopTunnelHandler(w http.ResponseWriter, _ *http.Request, id string) {
	if err := tunnel.StopTunnel(id); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ssoLoginTunnelHandler(w http.ResponseWriter, _ *http.Request, id string) {
	tunnels, err := tunnel.GetTunnels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, t := range tunnels {
		if t.ID == id {
			sess, err := tunnel.StartSSOLogin(t)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sess)
			return
		}
	}

	http.Error(w, "Tunnel not found", http.StatusNotFound)
}

func ssoStatusTunnelHandler(w http.ResponseWriter, _ *http.Request, id string) {
	tunnels, err := tunnel.GetTunnels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, t := range tunnels {
		if t.ID == id {
			profile := strings.TrimSpace(t.AWSProfile)
			if profile == "" {
				profile = "default"
			}

			isAuth := tunnel.CheckSSOAuthenticated(profile)
			resp := map[string]interface{}{
				"authenticated": isAuth,
			}

			if sess, exists := tunnel.GetSSOSession(id); exists {
				resp["auth_url"] = sess.AuthURL
				resp["user_code"] = sess.UserCode
				resp["done"] = sess.Done
				resp["error"] = sess.Error
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
	}

	http.Error(w, "Tunnel not found", http.StatusNotFound)
}
