package state

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/shacuicui/dispaccio/internal/check"
	"github.com/shacuicui/dispaccio/internal/crypto"
)

type Store struct {
	dir    string
	cipher *crypto.Cipher
}

func New(dir string, cipher *crypto.Cipher) *Store {
	return &Store{dir: dir, cipher: cipher}
}

func (s *Store) path(watchID string) string {
	sum := sha256.Sum256([]byte(watchID))
	name := hex.EncodeToString(sum[:8])
	return filepath.Join(s.dir, name+".dat")
}

func (s *Store) Load(watchID string) (check.Snapshot, error) {
	data, err := os.ReadFile(s.path(watchID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.cipher.Decrypt(data)
}

func (s *Store) Save(watchID string, snap check.Snapshot) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	out, err := s.cipher.Encrypt(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(watchID), out, 0o644)
}

func (s *Store) SaveArtifact(relPath string, data []byte) (bool, error) {
	full := filepath.Join(s.dir, filepath.FromSlash(relPath))
	if _, err := os.Stat(full); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return false, err
	}
	out, err := s.cipher.Encrypt(data)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(full, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
