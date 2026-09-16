package main

import (
	"fmt"
	"log"
	"net/http"

	"gopenvpn/api"
	"gopenvpn/config"
	"gopenvpn/pkg/proxy"
	"gopenvpn/pkg/vpn"
	"gopenvpn/web"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	proxy.LoadProxies()
	vpn.StartSystemServices()

	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/start", api.StartHandler(cfg))
	mux.HandleFunc("/api/stop", api.StopHandler())
	mux.HandleFunc("/api/status", api.StatusHandler())
	mux.HandleFunc("/api/proxies", api.ProxiesHandler())
	mux.HandleFunc("/api/tunnels", api.TunnelsHandler(cfg))
	mux.HandleFunc("/api/tunnels/", api.TunnelsHandler(cfg))

	// Web UI static files
	mux.Handle("/", http.FileServer(http.FS(web.FS)))

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting server on port %d...", cfg.Port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
