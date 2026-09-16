package tunnel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type TunnelType string

const (
	TypeAWSSSM TunnelType = "aws-ssm"
	TypeSSH    TunnelType = "ssh"
)

// Tunnel defines the configuration for a single port-forwarding connection.
type Tunnel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TunnelType        `json:"type"`
	AWSProfile  string            `json:"aws_profile,omitempty"`
	TargetTags  map[string]string `json:"target_tags,omitempty"`
	JumpHost    string            `json:"jump_host,omitempty"`
	JumpUser    string            `json:"jump_user,omitempty"`
	SSHKeyFile  string            `json:"ssh_key_file,omitempty"`
	RemoteHost  string            `json:"remote_host"`
	RemotePort  int               `json:"remote_port"`
	LocalPort   int               `json:"local_port"`
	Status      string            `json:"status,omitempty"` // "stopped", "connected", "error"
}

var (
	tunnelsFile = filepath.Join("data", "tunnels.json")
	mu          sync.RWMutex
)

// GetTunnels reads all tunnels from the JSON file.
func GetTunnels() ([]Tunnel, error) {
	mu.RLock()
	defer mu.RUnlock()

	file, err := os.Open(tunnelsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Tunnel{}, nil // Return empty list if file doesn't exist yet
		}
		return nil, err
	}
	defer file.Close()

	var tunnels []Tunnel
	if err := json.NewDecoder(file).Decode(&tunnels); err != nil {
		return nil, err
	}
	return tunnels, nil
}

// AddTunnel appends a new tunnel to the JSON file.
func AddTunnel(t Tunnel) error {
	tunnels, err := GetTunnels()
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	tunnels = append(tunnels, t)
	return saveTunnels(tunnels)
}

// UpdateTunnel updates an existing tunnel's configuration.
func UpdateTunnel(id string, updated Tunnel) error {
	tunnels, err := GetTunnels()
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for i, t := range tunnels {
		if t.ID == id {
			updated.ID = id // ensure ID is not mutated
			tunnels[i] = updated
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("tunnel not found")
	}

	return saveTunnels(tunnels)
}

// DeleteTunnel removes a tunnel from the JSON file.
func DeleteTunnel(id string) error {
	tunnels, err := GetTunnels()
	if err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	var filtered []Tunnel
	for _, t := range tunnels {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == len(tunnels) {
		return fmt.Errorf("tunnel not found")
	}

	return saveTunnels(filtered)
}

// saveTunnels marshals the array and writes it to disk. (Must be called within mu.Lock)
func saveTunnels(tunnels []Tunnel) error {
	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tunnels, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tunnelsFile, data, 0644)
}
