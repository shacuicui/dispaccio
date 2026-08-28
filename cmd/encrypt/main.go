// STATE_KEY=... go run ./cmd/encrypt -in config/watches.json -out config/watches.dat
package main

import (
	"flag"
	"log"
	"os"

	"github.com/shacuicui/dispaccio/internal/crypto"
)

func main() {
	in := flag.String("in", "config/watches.json", "clear-text JSON input")
	out := flag.String("out", "config/watches.dat", "encrypted output")
	flag.Parse()

	key := os.Getenv("STATE_KEY")
	if key == "" {
		log.Fatal("STATE_KEY is required")
	}
	cipher, err := crypto.New(key)
	if err != nil {
		log.Fatal(err)
	}
	if !cipher.Enabled {
		log.Fatal("STATE_KEY produced a disabled cipher")
	}

	plain, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, enc, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d bytes)", *out, len(enc))
}
