package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gopenvpn/pkg/proxy"
)

func ProxiesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func getProxiesHandler(w http.ResponseWriter, _ *http.Request) {
	rules := proxy.GetProxies()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func addProxyHandler(w http.ResponseWriter, r *http.Request) {
	var rule proxy.ProxyRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if rule.Port < 1024 || rule.Target == "" {
		http.Error(w, "Invalid port (must be >= 1024) or missing target", http.StatusBadRequest)
		return
	}

	if err := proxy.AddProxy(rule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rule)
}

func deleteProxyHandler(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	if err := proxy.DeleteProxy(port); err != nil {
		if err.Error() == "Proxy not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
