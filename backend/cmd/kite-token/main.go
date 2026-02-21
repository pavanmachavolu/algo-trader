// kite-token exchanges a Kite request_token for an access_token.
// Starts HTTP server on port 8888, opens login URL in browser, captures redirect
// with request_token, exchanges for access_token, saves to .env.
//
// Set redirect URL to http://localhost:8888 in your Kite app settings.
//
// Usage: go run ./cmd/kite-token
package main

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

const callbackPort = "8888"

func main() {
	envPaths := []string{".env", "backend/.env"}
	for _, p := range envPaths {
		_ = godotenv.Load(p)
	}

	apiKey := os.Getenv("KITE_API_KEY")
	apiSecret := os.Getenv("KITE_API_SECRET")

	if apiKey == "" || apiSecret == "" {
		log.Fatal("KITE_API_KEY and KITE_API_SECRET are required in .env")
	}

	kc := kiteconnect.New(apiKey)
	loginURL := kc.GetLoginURL()

	// Channel to receive access token from callback handler
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

		envPath := findEnvPath()
		if err := saveAccessTokenToEnv(envPath, data.AccessToken); err != nil {
			errCh <- err
			sendHTML(w, false, "Saved token but failed to write .env: "+err.Error())
			return
		}

		done <- data.AccessToken
		sendHTML(w, true, fmt.Sprintf("Success! Access token saved to %s", envPath))
	})

	server := &http.Server{Addr: ":" + callbackPort, Handler: mux}

	go func() {
		log.Printf("Listening on http://localhost:%s (set this as redirect URL in Kite app)", callbackPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Open browser
	if err := openBrowser(loginURL); err != nil {
		log.Printf("Could not open browser: %v. Open manually: %s", err, loginURL)
	} else {
		log.Println("Opened login URL in browser. Complete login to continue.")
	}

	// Wait for callback or error
	select {
	case accessToken := <-done:
		log.Println("\nAccess token:", accessToken)
		log.Println("Saved to .env")
	case err := <-errCh:
		log.Fatalf("Error: %v", err)
	case <-time.After(2 * time.Minute):
		log.Fatal("Timeout waiting for login callback")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
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

func findEnvPath() string {
	for _, p := range []string{".env", "backend/.env"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}

func saveAccessTokenToEnv(envPath string, accessToken string) error {
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
