package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ActiveTunnel tracks the state of a running tunnel process.
type ActiveTunnel struct {
	Cancel context.CancelFunc
	Status string // "connecting", "connected", "error"
}

var (
	activeTunnels = make(map[string]*ActiveTunnel)
	managerMu     sync.RWMutex
)

// GetStatus returns the memory status of a tunnel. If not running, returns "stopped".
func GetStatus(id string) string {
	managerMu.RLock()
	defer managerMu.RUnlock()

	if t, exists := activeTunnels[id]; exists {
		return t.Status
	}
	return "stopped"
}

// setStatus updates the memory status of a tunnel.
func setStatus(id, status string) {
	managerMu.Lock()
	defer managerMu.Unlock()
	if t, exists := activeTunnels[id]; exists {
		t.Status = status
	}
}

// StartTunnel provisions necessary files (AWS creds) and launches the tunnel process.
func StartTunnel(t Tunnel, awsCredsB64 string) error {
	managerMu.Lock()
	defer managerMu.Unlock()

	if active, exists := activeTunnels[t.ID]; exists {
		if active.Status == "connecting" || active.Status == "connected" {
			return fmt.Errorf("tunnel %s is already running", t.ID)
		}
		// Clean up previous context if it ended in error/stopped
		if active.Cancel != nil {
			active.Cancel()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	activeTunnels[t.ID] = &ActiveTunnel{
		Cancel: cancel,
		Status: "connecting",
	}

	go runTunnel(ctx, t, awsCredsB64)
	return nil
}

// StopTunnel cancels the context of a running tunnel, effectively killing the process.
func StopTunnel(id string) error {
	managerMu.Lock()
	defer managerMu.Unlock()

	if active, exists := activeTunnels[id]; exists {
		active.Cancel()
		delete(activeTunnels, id)
		return nil
	}
	return fmt.Errorf("tunnel not running")
}

// runTunnel handles the actual OS execution and blocking.
func runTunnel(ctx context.Context, t Tunnel, awsCredsB64 string) {
	var cmd *exec.Cmd

	switch t.Type {
	case TypeAWSSSM:
		// Prepare AWS environment
		credsPath, err := prepareAWSCredentials(awsCredsB64)
		if err != nil {
			setStatus(t.ID, fmt.Sprintf("error: credentials: %v", err))
			return
		}

		profile := "default"
		if t.AWSProfile != "" {
			profile = t.AWSProfile
		}

		target, err := resolveAWSTarget(ctx, t.TargetTags, profile, credsPath)
		if err != nil {
			log.Printf("[Tunnel %s] %v", t.ID, err)
			setStatus(t.ID, fmt.Sprintf("error: target resolution: %v", err))
			return
		}
		log.Printf("[Tunnel %s] Using AWS EC2 Instance %s for tunnel %s", t.ID, target, t.Name)

		ssmPort, err := getFreePort()
		if err != nil {
			setStatus(t.ID, fmt.Sprintf("error allocating ephemeral port: %v", err))
			return
		}

		params := fmt.Sprintf("host=%s,portNumber=%d,localPortNumber=%d", t.RemoteHost, t.RemotePort, ssmPort)
		cmd = exec.CommandContext(ctx, "aws", "ssm", "start-session",
			"--profile", profile,
			"--target", target,
			"--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
			"--parameters", params,
		)

		// Inject the dynamically decoded AWS_CREDENTIALS file
		cmd.Env = append(os.Environ(), "AWS_SHARED_CREDENTIALS_FILE="+credsPath)

		// Start TCP forwarder to expose 127.0.0.1 (SSM) to 0.0.0.0
		go tcpForward(ctx, fmt.Sprintf("0.0.0.0:%d", t.LocalPort), fmt.Sprintf("127.0.0.1:%d", ssmPort))

	case TypeSSH:
		args := []string{
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "BatchMode=yes",
			"-N", "-L",
			fmt.Sprintf("0.0.0.0:%d:%s:%d", t.LocalPort, t.RemoteHost, t.RemotePort),
		}

		keyPath, err := prepareSSHKey(t.SSHKeyFile)
		if err != nil {
			setStatus(t.ID, fmt.Sprintf("error: %v", err))
			return
		}
		if keyPath != "" {
			defer os.Remove(keyPath)
			args = append(args, "-i", keyPath)
		}

		args = append(args, fmt.Sprintf("%s@%s", t.JumpUser, t.JumpHost))
		cmd = exec.CommandContext(ctx, "ssh", args...)
	default:
		setStatus(t.ID, "error: unknown tunnel type")
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[Tunnel %s] Starting %s tunnel to %s...", t.ID, t.Type, t.RemoteHost)
	setStatus(t.ID, "connected")

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			// context canceled, stopped cleanly
			log.Printf("[Tunnel %s] Stopped cleanly", t.ID)
			return
		}
		log.Printf("[Tunnel %s] Error: %v", t.ID, err)
		setStatus(t.ID, fmt.Sprintf("error: %v", err))
	} else {
		// exited on its own
		log.Printf("[Tunnel %s] Exited on its own", t.ID)
		setStatus(t.ID, "stopped")
	}
}

// prepareAWSCredentials decodes AWS_CREDENTIALS base64 string into a temp file for the aws-cli to use.
func prepareAWSCredentials(b64Creds string) (string, error) {
	if b64Creds == "" {
		// If empty, return empty path and let AWS CLI use its default auth chain
		return "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(b64Creds)
	if err != nil {
		return "", fmt.Errorf("failed to decode AWS_CREDENTIALS base64: %v", err)
	}

	tmpFile := filepath.Join(os.TempDir(), "gopenvpn_aws_credentials")
	if err := os.WriteFile(tmpFile, decoded, 0600); err != nil {
		return "", err
	}

	return tmpFile, nil
}

// prepareSSHPrivateKey reads the ssh key from /data/ssh_keys, and copies it to a temp file with 0600 permissions
func prepareSSHKey(keyFilename string) (string, error) {
	if keyFilename == "" {
		return "", nil
	}

	origPath := filepath.Join("/data", "ssh_keys", keyFilename)
	data, err := os.ReadFile(origPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ssh key %s: %v", keyFilename, err)
	}

	b := make([]byte, 16)
	rand.Read(b)
	tmpFile := filepath.Join(os.TempDir(), "gopenvpn_ssh_"+hex.EncodeToString(b))

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write secure ssh key temp file: %v", err)
	}

	return tmpFile, nil
}

// resolveAWSTarget uses the AWS CLI to find a running EC2 instance matching the provided tags.
func resolveAWSTarget(ctx context.Context, tags map[string]string, profile, credsPath string) (string, error) {
	if len(tags) == 0 {
		return "", fmt.Errorf("no target tags provided to find the SSM target")
	}

	args := []string{"ec2", "describe-instances"}
	if profile != "default" && profile != "" {
		args = append(args, "--profile", profile)
	}

	args = append(args, "--filters")
	for k, v := range tags {
		args = append(args, fmt.Sprintf("Name=tag:%s,Values=%s", k, v))
	}
	args = append(args, "Name=instance-state-name,Values=running")
	args = append(args, "--query", "Reservations[0].Instances[0].InstanceId", "--output", "text")

	cmd := exec.CommandContext(ctx, "aws", args...)
	cmd.Env = append(os.Environ(), "AWS_SHARED_CREDENTIALS_FILE="+credsPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to query EC2 instances: %s - %v", strings.TrimSpace(string(out)), err)
	}

	instanceID := strings.TrimSpace(string(out))
	if instanceID == "None" || instanceID == "null" || instanceID == "" {
		return "", fmt.Errorf("no running EC2 instances found matching the provided tags")
	}

	return instanceID, nil
}

// getFreePort requests a random open port from the OS on the loopback interface.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// tcpForward handles port forwarding from a local listening address (e.g. 0.0.0.0:port)
// to a destination address (e.g. 127.0.0.1:ssmPort).
func tcpForward(ctx context.Context, listenAddr, dialAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Printf("[TCP Forward] Error listening on %s: %v", listenAddr, err)
		return
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return // triggered when listener is closed by context
		}

		go func(src net.Conn) {
			defer src.Close()
			dst, err := net.Dial("tcp", dialAddr)
			if err != nil {
				log.Printf("[TCP Forward] Error dialing %s: %v", dialAddr, err)
				return
			}
			defer dst.Close()

			// Bi-directional copy with sync
			errc := make(chan error, 2)
			go func() {
				_, err := io.Copy(dst, src)
				errc <- err
			}()
			go func() {
				_, err := io.Copy(src, dst)
				errc <- err
			}()
			
			<-errc // Wait for first io.Copy to finish (either EOF or error)
		}(conn)
	}
}

// StopAllTunnels stops all running tunnels
func StopAllTunnels() {
	managerMu.Lock()
	defer managerMu.Unlock()
	for id, active := range activeTunnels {
		if active.Cancel != nil {
			active.Cancel()
		}
		delete(activeTunnels, id)
	}
}
