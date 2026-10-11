package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	service := newRoomService(&realtimeClient{key: key, base: infraiBase, http: &http.Client{}})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rooms/join", func(w http.ResponseWriter, r *http.Request) {
		var in joinInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		out, err := service.join(r.Context(), in)
		if err != nil {
			respondError(w, err)
			return
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /rooms/payment", func(w http.ResponseWriter, r *http.Request) {
		var in paymentInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		out, err := service.payment(r.Context(), in)
		if err != nil {
			respondError(w, err)
			return
		}
		writeJSON(w, out)
	})
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, err error) {
	var api *APIError
	if errors.As(err, &api) {
		status := http.StatusBadGateway
		if api.Status >= 400 && api.Status < 500 {
			status = api.Status
		}
		http.Error(w, api.Error(), status)
		return
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		http.Error(w, "request deadline exceeded", 504)
		return
	}
	var domain domainError
	if errors.As(err, &domain) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "upstream request failed", http.StatusBadGateway)
}
