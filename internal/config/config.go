package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/shacuicui/dispaccio/internal/crypto"
)

type Watch struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Label  string          `json:"label"`
	Config json.RawMessage `json:"config"`
}

func Load(path string, cipher *crypto.Cipher) ([]Watch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	plain, err := cipher.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("config: decrypt %s: %w", path, err)
	}

	var watches []Watch
	if err := json.Unmarshal(plain, &watches); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	seen := make(map[string]struct{}, len(watches))
	for _, w := range watches {
		if w.ID == "" || w.Type == "" {
			return nil, fmt.Errorf("config: every watch needs an id and a type")
		}
		if _, dup := seen[w.ID]; dup {
			return nil, fmt.Errorf("config: duplicate watch id %q", w.ID)
		}
		seen[w.ID] = struct{}{}
	}
	return watches, nil
}
