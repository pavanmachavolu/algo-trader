package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"encoding/json"

	"algo-trader/backend/database"
	"algo-trader/backend/handlers"
	"algo-trader/backend/pkg/kite"
	"algo-trader/backend/pkg/notify"
	"algo-trader/backend/services"

	"github.com/gorilla/mux"
)

func main() {
	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Reload Telegram config after env is loaded by price fetcher
	notify.Reload()

	// Start price fetcher service in background
	go services.StartPriceFetcher()

	// Start virtual trader scheduler
	go services.StartTrader()

	// Setup routes
	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()

	api.HandleFunc("/stocks", handlers.GetStocks).Methods("GET")
	api.HandleFunc("/stocks/{symbol}/history", handlers.GetStockHistory).Methods("GET")
	api.HandleFunc("/positions", handlers.GetPositions).Methods("GET")
	api.HandleFunc("/positions/summary", handlers.GetPositionsSummary).Methods("GET")
	api.HandleFunc("/trades", handlers.GetTrades).Methods("GET")

	api.HandleFunc("/reload-config", func(w http.ResponseWriter, r *http.Request) {
		kite.LoadEnv()
		notify.Reload()
		token := os.Getenv("KITE_ACCESS_TOKEN")
		masked := ""
		if len(token) > 6 {
			masked = token[:3] + "..." + token[len(token)-3:]
		}
		resp := map[string]string{
			"status":            "reloaded",
			"kite_access_token": masked,
			"telegram_chat_id":  os.Getenv("TELEGRAM_CHAT_ID"),
		}
		log.Println("Config reloaded from .env")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(resp)
	}).Methods("POST")

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	port := getEnv("PORT", "8080")
	log.Printf("Server starting on port %s", port)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := http.ListenAndServe(":"+port, router); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
