package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webreader/config"
	"webreader/handler"
	"webreader/pool"
	"webreader/provider"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("[INFO] Starting webreader service...")

	cfg := config.LoadConfig()
	log.Printf("[INFO] Config loaded: port=%s, default_provider=%s, timeout=%ds (max %ds), concurrency=%d, rpm_limit=%d",
		cfg.Port, cfg.DefaultProvider, cfg.DefaultTimeoutSecs, cfg.MaxTimeoutSecs, cfg.MaxConcurrentRequests, cfg.MaxRequestsPerMinute)

	registry := provider.NewRegistry(cfg.DefaultProvider)

	// Register providers
	jinaProvider := provider.NewJinaProvider(cfg.JinaAPIKey)
	registry.Register(jinaProvider)

	// Initialize worker pool & rate limiter
	limiter := pool.NewLimiter(cfg.MaxConcurrentRequests, cfg.MaxRequestsPerMinute)

	h := handler.NewHandler(cfg, registry, limiter)

	mux := http.NewServeMux()

	// Probes / Health endpoints
	mux.HandleFunc("/ping", h.HealthHandler)
	mux.HandleFunc("/ok", h.HealthHandler)
	mux.HandleFunc("/health", h.HealthHandler)

	// API endpoints
	mux.HandleFunc("/v1/status", h.StatusHandler)
	mux.HandleFunc("/v1/providers", h.ProvidersHandler)
	mux.HandleFunc("/v1/markdown", h.MarkdownHandler)
	mux.HandleFunc("/raw", h.RawMarkdownHandler)

	// Top-level root redirects / helps
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":"webreader","version":"1.0.0","status":"running","docs":"/v1/markdown?url=<target_url>"}`)
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: time.Duration(cfg.MaxTimeoutSecs+10) * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run context for graceful shutdown
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("[FATAL] Graceful shutdown timed out.. forcing exit.")
			}
		}()

		log.Println("[INFO] Shutting down webreader HTTP server...")
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatalf("[ERROR] Server shutdown error: %v", err)
		}
		serverStopCtx()
	}()

	log.Printf("[INFO] webreader is listening on :%s", cfg.Port)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("[FATAL] ListenAndServe error: %v", err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
	log.Println("[INFO] Server exited successfully.")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/ping" && r.URL.Path != "/ok" && r.URL.Path != "/health" {
			log.Printf("[REQ] %s %s from %s in %s", r.Method, r.URL.RequestURI(), r.RemoteAddr, time.Since(start))
		}
	})
}
