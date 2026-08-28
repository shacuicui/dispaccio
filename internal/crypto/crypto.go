package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var magic = []byte("SHACUI鲨脆")

const formatVersion byte = 1

type Cipher struct {
	Enabled bool
	aead    cipher.AEAD
}

func New(key string) (*Cipher, error) {
	if key == "" {
		return &Cipher{Enabled: false}, nil
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}
	return &Cipher{Enabled: true, aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	if !c.Enabled {
		return plaintext, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, plaintext, nil)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(sealed)))
	base64.StdEncoding.Encode(encoded, sealed)

	out := make([]byte, 0, len(magic)+1+len(encoded))
	out = append(out, magic...)
	out = append(out, formatVersion)
	out = append(out, encoded...)
	return out, nil
}

func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	if !c.Enabled {
		return data, nil
	}

	headerLen := len(magic) + 1
	if len(data) < headerLen || !bytes.Equal(data[:len(magic)], magic) {
		return nil, errors.New("crypto: not a dispaccio file (bad magic)")
	}
	if v := data[len(magic)]; v != formatVersion {
		return nil, fmt.Errorf("crypto: unsupported format version %d", v)
	}
	data = data[headerLen:]

	sealed := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(sealed, data)
	if err != nil {
		return nil, fmt.Errorf("crypto: base64: %w", err)
	}
	sealed = sealed[:n]
	if len(sealed) < c.aead.NonceSize() {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce := sealed[:c.aead.NonceSize()]
	ciphertext := sealed[c.aead.NonceSize():]
	return c.aead.Open(nil, nonce, ciphertext, nil)
}
