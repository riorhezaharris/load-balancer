package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	name        = envOrDefault("BACKEND_NAME", "backend")
	baseDelayMs = int64(mustAtoi(envOrDefault("BASE_DELAY_MS", "0")))
	extraDelay  atomic.Int64 // added by /degrade, cleared by /recover
)

func main() {
	port := envOrDefault("PORT", "3000")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /degrade", handleDegrade)
	mux.HandleFunc("POST /recover", handleRecover)
	mux.HandleFunc("/", handleRequest)

	log.Printf("%s listening on :%s (base delay: %dms)", name, port, baseDelayMs)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	total := baseDelayMs + extraDelay.Load()
	if total > 0 {
		time.Sleep(time.Duration(total) * time.Millisecond)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"backend": name,
		"path":    r.URL.Path,
		"delay_ms": total,
	})
}

func handleDegrade(w http.ResponseWriter, _ *http.Request) {
	extraDelay.Store(500)
	log.Printf("%s degraded (+500ms)", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "degraded", "extra_delay_ms": "500"})
}

func handleRecover(w http.ResponseWriter, _ *http.Request) {
	extraDelay.Store(0)
	log.Printf("%s recovered", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recovered"})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
