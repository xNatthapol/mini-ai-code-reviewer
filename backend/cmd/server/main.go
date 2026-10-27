package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/api"
	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/groq"
	"github.com/xNatthapol/mini-ai-code-reviewer/backend/internal/review"
)

type config struct {
	apiKey string
	model  string
	port   string
}

func readConfig() (config, error) {
	cfg := config{
		apiKey: strings.TrimSpace(os.Getenv("GROQ_API_KEY")),
		model:  strings.TrimSpace(os.Getenv("GROQ_MODEL")),
		port:   strings.TrimSpace(os.Getenv("PORT")),
	}
	if cfg.apiKey == "" {
		return config{}, errors.New("GROQ_API_KEY is required")
	}
	if cfg.model == "" {
		cfg.model = "openai/gpt-oss-120b"
	}
	if cfg.port == "" {
		cfg.port = "8080"
	}
	port, err := strconv.Atoi(cfg.port)
	if err != nil || port < 1 || port > 65535 {
		return config{}, errors.New("PORT must be an integer from 1 to 65535")
	}
	return cfg, nil
}

func newServer(cfg config, reviewer review.Reviewer) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /lint", api.NewHandler(reviewer))
	return &http.Server{
		Addr:              "127.0.0.1:" + cfg.port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Leave time to send the response after the provider's 30-second deadline.
		WriteTimeout: 45 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func main() {
	cfg, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}
	reviewer := review.NewCachedReviewer(groq.NewClient(&http.Client{}, cfg.apiKey, cfg.model))
	server := newServer(cfg, reviewer)
	log.Printf("Listening on http://%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
