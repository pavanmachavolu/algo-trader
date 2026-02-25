package kite

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	kiteconnect "github.com/zerodhatech/gokiteconnect"
	"github.com/joho/godotenv"
)

const CallbackPort = "8888"

// LoadEnv loads .env from standard project locations, overwriting any
// existing values so that runtime reloads pick up file changes.
func LoadEnv() {
	for _, p := range []string{".env", "backend/.env", "/root/.env"} {
		_ = godotenv.Overload(p)
	}
}

// ValidateToken checks whether the given access token is still valid
// by making a lightweight GetUserProfile call.
func ValidateToken(apiKey, accessToken string) bool {
	kc := kiteconnect.New(apiKey)
	kc.SetAccessToken(accessToken)
	_, err := kc.GetUserProfile()
	return err == nil
}

// EnsureAccessToken returns a valid access token. It first checks the
// KITE_ACCESS_TOKEN env var; if present and valid, returns it immediately.
// Otherwise it runs the interactive browser login flow, saves the new token
// to .env and os.Setenv, and returns it.
func EnsureAccessToken(apiKey, apiSecret string) (string, error) {
	existing := os.Getenv("KITE_ACCESS_TOKEN")
	if existing != "" {
		if ValidateToken(apiKey, existing) {
			return existing, nil
		}
		log.Println("Existing KITE_ACCESS_TOKEN is invalid/expired, starting login flow...")
	} else {
		log.Println("No KITE_ACCESS_TOKEN set, starting login flow...")
	}

	token, err := RunLoginFlow(apiKey, apiSecret)
	if err != nil {
		return "", err
	}

	os.Setenv("KITE_ACCESS_TOKEN", token)
	return token, nil
}

// RunLoginFlow starts an HTTP server on CallbackPort, opens the Kite login
// URL in the browser, waits for the redirect callback with request_token,
// exchanges it for an access_token, saves it to .env, and returns it.
func RunLoginFlow(apiKey, apiSecret string) (string, error) {
	kc := kiteconnect.New(apiKey)
	loginURL := kc.GetLoginURL()

	done := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestToken := r.URL.Query().Get("request_token")
		if requestToken == "" {
			status := r.URL.Query().Get("status")
			msg := r.URL.Query().Get("message")
			sendHTML(w, false, fmt.Sprintf("No request_token. status=%s message=%s", status, msg))
			return
		}

		data, err := kc.GenerateSession(requestToken, apiSecret)
		if err != nil {
			errCh <- err
			sendHTML(w, false, "Failed to exchange token: "+err.Error())
			return
		}

		envPath := FindEnvPath()
		if err := SaveAccessTokenToEnv(envPath, data.AccessToken); err != nil {
			errCh <- err
			sendHTML(w, false, "Got token but failed to write .env: "+err.Error())
			return
		}

		done <- data.AccessToken
		sendHTML(w, true, fmt.Sprintf("Success! Access token saved to %s", envPath))
	})

	server := &http.Server{Addr: ":" + CallbackPort, Handler: mux}

	go func() {
		log.Printf("Listening on http://localhost:%s (set this as redirect URL in Kite app)", CallbackPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	if err := OpenBrowser(loginURL); err != nil {
		log.Printf("Could not open browser: %v\nOpen manually: %s", err, loginURL)
	} else {
		log.Println("Opened login URL in browser. Complete login to continue.")
	}

	var token string
	select {
	case token = <-done:
		log.Println("Access token obtained and saved to .env")
	case err := <-errCh:
		shutdownServer(server)
		return "", fmt.Errorf("login flow: %w", err)
	case <-time.After(2 * time.Minute):
		shutdownServer(server)
		return "", fmt.Errorf("timeout waiting for login callback (2m)")
	}

	shutdownServer(server)
	return token, nil
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// OpenBrowser opens the given URL in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func sendHTML(w http.ResponseWriter, success bool, msg string) {
	status := "Error"
	color := "#c00"
	if success {
		status = "Success"
		color = "#0a0"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Kite Token</title></head>
<body style="font-family:sans-serif;padding:2rem;text-align:center;">
<h2 style="color:%s">%s</h2>
<p>%s</p>
<p style="color:#666;font-size:0.9em">You can close this tab.</p>
</body></html>`, color, status, msg)
}

// FindEnvPath returns the first existing .env file path from standard locations.
func FindEnvPath() string {
	for _, p := range []string{".env", "backend/.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}

// SaveAccessTokenToEnv replaces or appends the KITE_ACCESS_TOKEN line in the
// given .env file, along with a timestamp comment.
func SaveAccessTokenToEnv(envPath string, accessToken string) error {
	content, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	newLine := fmt.Sprintf("KITE_ACCESS_TOKEN=%s", accessToken)
	commentLine := fmt.Sprintf("# KITE_ACCESS_TOKEN generated at %s", ts)

	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	var out []string
	replaced := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "KITE_ACCESS_TOKEN=") {
			out = append(out, commentLine, newLine)
			replaced = true
			continue
		}
		out = append(out, line)
	}

	if !replaced {
		out = append(out, "", commentLine, newLine)
	}

	return os.WriteFile(envPath, []byte(strings.Join(out, "\n")+"\n"), 0600)
}
