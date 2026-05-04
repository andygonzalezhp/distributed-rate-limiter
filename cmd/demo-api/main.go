package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Response struct {
	Message   string `json:"message"`
	Route     string `json:"route"`
	Timestamp string `json:"timestamp"`
}

func writeJSON(w http.ResponseWriter, message string, route string) {
	resp := Response{
		Message:   message,
		Route:     route,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, "hello from backend API", "/hello")
	})

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, "search endpoint reached", "/search")
	})

	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, "admin endpoint reached", "/admin")
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend healthy"))
	})

	log.Println("Demo backend API running on :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal(err)
	}
}
