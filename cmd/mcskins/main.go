// Command mcskins runs the Minecraft skin HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tinybrickboy/mcskins/internal/server"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	addr := env("MCSKINS_ADDR", ":8080")
	ttl := time.Duration(envInt("MCSKINS_CACHE_TTL_SECONDS", 1800)) * time.Second
	proxies := envList("MCSKINS_PROXIES")

	srv := &http.Server{
		Addr: addr,
		Handler: server.New(server.Config{
			TTL:     ttl,
			Proxies: proxies,
		}, log).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", addr, "cache_ttl", ttl, "proxies", len(proxies))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envList parses a comma-separated env var into a trimmed, non-empty slice.
func envList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
