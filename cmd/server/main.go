package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/api"
)

func main() {
	mux := api.NewRouter()

	addr := os.Getenv("SERVER_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting server on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
