// Package server wires the Mojang client and renderer behind an HTTP API.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/tinybrickboy/mcskins/internal/mojang"
	"github.com/tinybrickboy/mcskins/internal/render"
)

const (
	defaultSize = 128
	maxSize     = 512
)

// Server holds dependencies for the HTTP handlers.
type Server struct {
	mc  *mojang.Client
	log *slog.Logger
}

// Config configures a Server.
type Config struct {
	Proxies []string // proxy URLs for 429 fallback (socks5/http)
}

// New builds a Server with a Mojang client from cfg.
func New(cfg Config, log *slog.Logger) *Server {
	return &Server{mc: mojang.New(cfg.Proxies), log: log}
}

// Routes returns the configured http.Handler.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /skin/{player}", s.handleSkin)
	mux.HandleFunc("GET /face/{player}", s.handleImage(render.Face))
	mux.HandleFunc("GET /head/{player}", s.handleImage(render.Head))
	mux.HandleFunc("GET /avatar/{player}", s.handleImage(render.Head))
	mux.HandleFunc("GET /body/{player}", s.handleImage(render.Body))
	mux.HandleFunc("GET /pfp/{player}", s.handleImage(render.Pfp))
	mux.HandleFunc("GET /3dpfp/{player}", s.handle3DPfp)
	mux.HandleFunc("GET /", s.handleIndex)
	return s.recover(s.logRequests(mux))
}

type renderFunc func(img image.Image, size int) ([]byte, error)

func (s *Server) handleImage(fn renderFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		player := r.PathValue("player")
		skin, err := s.mc.Skin(r.Context(), player)
		if err != nil {
			s.writeErr(w, err)
			return
		}
		img, err := png.Decode(bytes.NewReader(skin.PNG))
		if err != nil {
			http.Error(w, "invalid skin texture", http.StatusBadGateway)
			return
		}
		out, err := fn(img, parseSize(r))
		if err != nil {
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}
		writePNG(w, out)
	}
}

func (s *Server) handleSkin(w http.ResponseWriter, r *http.Request) {
	skin, err := s.mc.Skin(r.Context(), r.PathValue("player"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	w.Header().Set("X-Skin-Model", skin.Model)
	writePNG(w, skin.PNG)
}

// handle3DPfp renders a stylized big-head 3D bust of the player's skin. Unlike
// the flat renders it needs the slim flag for correct arm width, so it has its
// own handler instead of going through handleImage.
func (s *Server) handle3DPfp(w http.ResponseWriter, r *http.Request) {
	skin, err := s.mc.Skin(r.Context(), r.PathValue("player"))
	if err != nil {
		s.writeErr(w, err)
		return
	}
	img, err := png.Decode(bytes.NewReader(skin.PNG))
	if err != nil {
		http.Error(w, "invalid skin texture", http.StatusBadGateway)
		return
	}
	out, err := render.Tiny3D(img, parseSize(r), skin.Slim)
	if err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	writePNG(w, out)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": "mcskins",
		"endpoints": []string{
			"GET /skin/{player}",
			"GET /face/{player}?size=N",
			"GET /head/{player}?size=N",
			"GET /avatar/{player}?size=N",
			"GET /body/{player}?size=N",
			"GET /pfp/{player}?size=N",
			"GET /3dpfp/{player}?size=N",
			"GET /health",
		},
		"notes": "player = username or UUID; size 1-512 (default 128)",
	})
}

func parseSize(r *http.Request) int {
	q := r.URL.Query().Get("size")
	if q == "" {
		return defaultSize
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 1 {
		return defaultSize
	}
	if n > maxSize {
		return maxSize
	}
	return n
}

func (s *Server) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, mojang.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "player not found"})
	case errors.Is(err, mojang.ErrRateLimited):
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limited; no peer available"})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, map[string]string{"error": "upstream timeout"})
	default:
		s.log.Error("upstream error", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream error"})
	}
}

func writePNG(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "err", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
