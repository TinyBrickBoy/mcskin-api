// Package mojang resolves Minecraft usernames/UUIDs to player skins via the
// official Mojang APIs, with in-memory caching and a sane default skin
// fallback.
package mojang

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tinybrickboy/mcskins/internal/cache"
)

// ErrNotFound is returned when a username or UUID does not resolve to a player.
var ErrNotFound = errors.New("player not found")

// Skin holds a fetched skin texture and its metadata.
type Skin struct {
	PNG   []byte // raw PNG bytes of the skin texture
	Slim  bool   // true for the "slim" (Alex) 3px-arm model
	Model string // "classic" or "slim"
}

// Client talks to the Mojang APIs. The zero value is not usable; use New.
type Client struct {
	http     *http.Client
	profiles *cache.TTL[string, profile] // key: lowercased name or uuid
	skins    *cache.TTL[string, *Skin]   // key: texture url
}

type profile struct {
	ID   string
	Name string
}

// New returns a Client with the given cache TTL for profiles and skins.
func New(ttl time.Duration) *Client {
	return &Client{
		http:     &http.Client{Timeout: 8 * time.Second},
		profiles: cache.New[string, profile](ttl),
		skins:    cache.New[string, *Skin](ttl),
	}
}

// Skin resolves player (username or UUID, with or without dashes) and returns
// its skin texture. Falls back to the default Steve/Alex skin if the player has
// none.
func (c *Client) Skin(ctx context.Context, player string) (*Skin, error) {
	id, err := c.resolveUUID(ctx, player)
	if err != nil {
		return nil, err
	}
	texURL, slim, err := c.textureURL(ctx, id)
	if err != nil {
		return nil, err
	}
	if texURL == "" {
		return defaultSkin(id), nil
	}
	if s, ok := c.skins.Get(texURL); ok {
		return s, nil
	}
	png, err := c.fetch(ctx, texURL)
	if err != nil {
		return nil, err
	}
	s := &Skin{PNG: png, Slim: slim, Model: model(slim)}
	c.skins.Set(texURL, s)
	return s, nil
}

// resolveUUID turns a username into a dash-less UUID, or normalizes a UUID.
func (c *Client) resolveUUID(ctx context.Context, player string) (string, error) {
	p := strings.ToLower(strings.TrimSpace(player))
	clean := strings.ReplaceAll(p, "-", "")
	if isUUID(clean) {
		return clean, nil
	}
	if pr, ok := c.profiles.Get(p); ok {
		return pr.ID, nil
	}
	u := "https://api.mojang.com/users/profiles/minecraft/" + url(player)
	body, status, err := c.get(ctx, u)
	if err != nil {
		return "", err
	}
	if status == http.StatusNoContent || status == http.StatusNotFound {
		return "", ErrNotFound
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("mojang: unexpected status %d", status)
	}
	var pr profile
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", err
	}
	if pr.ID == "" {
		return "", ErrNotFound
	}
	c.profiles.Set(p, pr)
	return pr.ID, nil
}

// textureURL fetches the session profile for id and extracts the skin texture
// URL and slim flag.
func (c *Client) textureURL(ctx context.Context, id string) (string, bool, error) {
	u := "https://sessionserver.mojang.com/session/minecraft/profile/" + id
	body, status, err := c.get(ctx, u)
	if err != nil {
		return "", false, err
	}
	if status == http.StatusNoContent || status == http.StatusNotFound {
		return "", false, ErrNotFound
	}
	if status != http.StatusOK {
		return "", false, fmt.Errorf("mojang: unexpected status %d", status)
	}
	var sp struct {
		Properties []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(body, &sp); err != nil {
		return "", false, err
	}
	for _, prop := range sp.Properties {
		if prop.Name != "textures" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(prop.Value)
		if err != nil {
			return "", false, err
		}
		var tex struct {
			Textures struct {
				Skin struct {
					URL      string `json:"url"`
					Metadata struct {
						Model string `json:"model"`
					} `json:"metadata"`
				} `json:"SKIN"`
			} `json:"textures"`
		}
		if err := json.Unmarshal(raw, &tex); err != nil {
			return "", false, err
		}
		return tex.Textures.Skin.URL, tex.Textures.Skin.Metadata.Model == "slim", nil
	}
	return "", false, nil
}

func (c *Client) fetch(ctx context.Context, u string) ([]byte, error) {
	body, status, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("mojang: texture fetch status %d", status)
	}
	return body, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "mcskins/1.0 (+https://github.com/tinybrickboy/mcskins)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func model(slim bool) string {
	if slim {
		return "slim"
	}
	return "classic"
}

func isUUID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// url percent-encodes a path segment minimally (Mojang names are safe ASCII,
// but guard against odd input).
func url(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), " ", "%20")
}
