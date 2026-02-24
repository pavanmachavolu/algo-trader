package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

var (
	botToken string
	chatID   string
)

func init() {
	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID = os.Getenv("TELEGRAM_CHAT_ID")
}

// Reload re-reads env vars (useful after godotenv.Load).
func Reload() {
	botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID = os.Getenv("TELEGRAM_CHAT_ID")
}

func configured() bool {
	return botToken != "" && chatID != ""
}

// Send sends a plain-text message to the configured Telegram chat.
func Send(message string) error {
	if !configured() {
		log.Printf("[telegram] not configured, skipping: %s", message)
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send: status %d", resp.StatusCode)
	}
	return nil
}

// Sendf sends a formatted message.
func Sendf(format string, args ...interface{}) error {
	return Send(fmt.Sprintf(format, args...))
}
