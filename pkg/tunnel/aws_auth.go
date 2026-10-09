package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SSOAuthSession tracks active AWS SSO login process
type SSOAuthSession struct {
	AuthURL   string    `json:"auth_url"`
	UserCode  string    `json:"user_code"`
	ExpiresAt time.Time `json:"expires_at"`
	Done      bool      `json:"done"`
	Error     string    `json:"error,omitempty"`
}

var (
	ssoSessions   = make(map[string]*SSOAuthSession)
	ssoSessionsMu sync.RWMutex

	reSSOURL  = regexp.MustCompile(`https://[^\s]+(?:device\?user_code=[A-Z0-9-]+|start/#/device\?user_code=[A-Z0-9-]+|device[^\s]*|amazonaws\.com/?[^\s]*)`)
	reSSOCode = regexp.MustCompile(`([A-Z0-9]{4}-[A-Z0-9]{4}|[A-Z0-9]{8})`)
)

// GetAWSDir returns the path to ~/.aws directory in a cross-platform way.
func GetAWSDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0700); err != nil {
		return "", err
	}
	return awsDir, nil
}

// EnsureAWSConfig writes or updates the ~/.aws/config and ~/.aws/credentials files
// for the given tunnel if it defines specific AWS parameters.
func EnsureAWSConfig(t Tunnel) error {
	if t.Type != TypeAWSSSM {
		return nil
	}

	profile := strings.TrimSpace(t.AWSProfile)
	if profile == "" {
		profile = "default"
	}

	awsDir, err := GetAWSDir()
	if err != nil {
		return fmt.Errorf("unable to access AWS directory: %v", err)
	}

	configFile := filepath.Join(awsDir, "config")
	credsFile := filepath.Join(awsDir, "credentials")

	if t.IsSSO {
		// Update ~/.aws/config with SSO profile
		return updateAWSConfigFileSSO(configFile, profile, t)
	}

	// Static credentials
	if t.AWSAccessKeyID != "" && t.AWSSecretAccessKey != "" {
		if err := updateAWSCredentialsFileStatic(credsFile, profile, t); err != nil {
			return err
		}
		if t.Region != "" {
			if err := updateAWSConfigFileRegion(configFile, profile, t.Region); err != nil {
				return err
			}
		}
	}

	return nil
}

func updateAWSConfigFileSSO(configFile, profile string, t Tunnel) error {
	sectionHeader := fmt.Sprintf("[profile %s]", profile)
	if profile == "default" {
		sectionHeader = "[default]"
	}

	region := t.Region
	if region == "" {
		region = t.SSORegion
	}

	lines := []string{
		sectionHeader,
		fmt.Sprintf("sso_start_url = %s", t.SSOStartURL),
		fmt.Sprintf("sso_region = %s", t.SSORegion),
		fmt.Sprintf("sso_account_id = %s", t.SSOAccountID),
		fmt.Sprintf("sso_role_name = %s", t.SSORoleName),
	}
	if region != "" {
		lines = append(lines, fmt.Sprintf("region = %s", region))
	}
	lines = append(lines, "output = json")

	return upsertINISection(configFile, sectionHeader, lines)
}

func updateAWSCredentialsFileStatic(credsFile, profile string, t Tunnel) error {
	sectionHeader := fmt.Sprintf("[%s]", profile)
	lines := []string{
		sectionHeader,
		fmt.Sprintf("aws_access_key_id = %s", t.AWSAccessKeyID),
		fmt.Sprintf("aws_secret_access_key = %s", t.AWSSecretAccessKey),
	}
	if t.AWSSessionToken != "" {
		lines = append(lines, fmt.Sprintf("aws_session_token = %s", t.AWSSessionToken))
	}
	return upsertINISection(credsFile, sectionHeader, lines)
}

func updateAWSConfigFileRegion(configFile, profile, region string) error {
	sectionHeader := fmt.Sprintf("[profile %s]", profile)
	if profile == "default" {
		sectionHeader = "[default]"
	}
	lines := []string{
		sectionHeader,
		fmt.Sprintf("region = %s", region),
		"output = json",
	}
	return upsertINISection(configFile, sectionHeader, lines)
}

// upsertINISection updates or appends a section in an INI file
func upsertINISection(filename, sectionHeader string, newSectionLines []string) error {
	var existingContent []byte
	if data, err := os.ReadFile(filename); err == nil {
		existingContent = data
	}

	scanner := bufio.NewScanner(bytes.NewReader(existingContent))
	var outLines []string
	inTargetSection := false
	replaced := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if strings.EqualFold(trimmed, sectionHeader) {
				inTargetSection = true
				replaced = true
				outLines = append(outLines, newSectionLines...)
				continue
			} else if inTargetSection {
				inTargetSection = false
			}
		}

		if !inTargetSection {
			outLines = append(outLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	if !replaced {
		if len(outLines) > 0 && strings.TrimSpace(outLines[len(outLines)-1]) != "" {
			outLines = append(outLines, "")
		}
		outLines = append(outLines, newSectionLines...)
	}

	content := strings.Join(outLines, "\n") + "\n"
	return os.WriteFile(filename, []byte(content), 0600)
}

// CheckSSOAuthenticated runs `aws sts get-caller-identity --profile <profile>` to verify if the profile has valid credentials
func CheckSSOAuthenticated(profile string) bool {
	if profile == "" {
		profile = "default"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity", "--profile", profile)
	err := cmd.Run()
	return err == nil
}

// GetSSOSession returns the current SSO auth session for a tunnel
func GetSSOSession(tunnelID string) (*SSOAuthSession, bool) {
	ssoSessionsMu.RLock()
	defer ssoSessionsMu.RUnlock()
	sess, exists := ssoSessions[tunnelID]
	if !exists {
		return nil, false
	}
	return sess, true
}

// StartSSOLogin triggers `aws sso login --profile <profile> --no-browser` and captures the verification link
func StartSSOLogin(t Tunnel) (*SSOAuthSession, error) {
	if t.Type != TypeAWSSSM || !t.IsSSO {
		return nil, fmt.Errorf("tunnel %s is not configured for AWS SSO", t.ID)
	}

	profile := strings.TrimSpace(t.AWSProfile)
	if profile == "" {
		profile = "default"
	}

	// Ensure config is written
	if err := EnsureAWSConfig(t); err != nil {
		return nil, fmt.Errorf("failed to prepare AWS config: %v", err)
	}

	ssoSessionsMu.Lock()
	sess := &SSOAuthSession{
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	ssoSessions[t.ID] = sess
	ssoSessionsMu.Unlock()

	setStatus(t.ID, "authenticating")

	cmd := exec.Command("aws", "sso", "login", "--profile", profile, "--no-browser")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ssoSessionsMu.Lock()
		sess.Error = err.Error()
		sess.Done = true
		ssoSessionsMu.Unlock()
		setStatus(t.ID, "error: "+err.Error())
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		ssoSessionsMu.Lock()
		sess.Error = err.Error()
		sess.Done = true
		ssoSessionsMu.Unlock()
		setStatus(t.ID, "error: "+err.Error())
		return nil, err
	}

	r := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(r)

	// Stream reader in background to capture auth URL & code
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[AWS SSO %s] %s", profile, line)

			// Try find URL
			if match := reSSOURL.FindString(line); match != "" {
				ssoSessionsMu.Lock()
				// If we find an upgraded URL with user_code parameter, prefer it
				if sess.AuthURL == "" || strings.Contains(match, "user_code=") {
					sess.AuthURL = strings.TrimSpace(match)
				}
				ssoSessionsMu.Unlock()
			}

			// Try find User Code from URL query param if present
			if sess.AuthURL != "" && strings.Contains(sess.AuthURL, "user_code=") {
				parts := strings.Split(sess.AuthURL, "user_code=")
				if len(parts) > 1 {
					codePart := strings.Split(parts[1], "&")[0]
					if match := reSSOCode.FindString(codePart); match != "" {
						ssoSessionsMu.Lock()
						sess.UserCode = match
						ssoSessionsMu.Unlock()
					}
				}
			}

			// Try find User Code from text output
			if sess.UserCode == "" {
				// AWS CLI typically prints: "Then enter the code:\n\nXXXX-XXXX" or "code: XXXX-XXXX"
				if strings.Contains(strings.ToLower(line), "code") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						codeCandidate := strings.TrimSpace(parts[1])
						if match := reSSOCode.FindString(codeCandidate); match != "" {
							ssoSessionsMu.Lock()
							sess.UserCode = match
							ssoSessionsMu.Unlock()
						}
					}
				} else {
					if match := reSSOCode.FindString(line); match != "" && !strings.Contains(line, "http") {
						ssoSessionsMu.Lock()
						sess.UserCode = match
						ssoSessionsMu.Unlock()
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[AWS SSO %s] Error reading process output: %v", profile, err)
		}

		// Wait for command completion
		waitErr := cmd.Wait()
		ssoSessionsMu.Lock()
		sess.Done = true
		if waitErr != nil {
			sess.Error = waitErr.Error()
			setStatus(t.ID, "needs_login")
		} else {
			log.Printf("[AWS SSO %s] Authentication successful!", profile)
			setStatus(t.ID, "stopped")
		}
		ssoSessionsMu.Unlock()
	}()

	// Wait up to 3 seconds to catch the URL and code immediately for initial API response
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ssoSessionsMu.RLock()
		hasURL := sess.AuthURL != ""
		hasCode := sess.UserCode != ""
		ssoSessionsMu.RUnlock()
		if hasURL && hasCode {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	return sess, nil
}
