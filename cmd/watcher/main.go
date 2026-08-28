// flags:
//
//	-config  path to the encrypted watches file
//	-state   directory for state files
//	-test    Discord test message
//	-list    print the configured watches
//	-report  send a status report from saved state, then exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shacuicui/dispaccio/internal/config"
	"github.com/shacuicui/dispaccio/internal/crypto"
	"github.com/shacuicui/dispaccio/internal/engine"
	"github.com/shacuicui/dispaccio/internal/notify"
	"github.com/shacuicui/dispaccio/internal/state"
)

func main() {
	configPath := flag.String("config", "config/watches.dat", "path to the encrypted watches file")
	stateDir := flag.String("state", "state", "directory for state files")
	testNotify := flag.Bool("test", false, "send a Discord test message and exit")
	list := flag.Bool("list", false, "print the configured watches and exit")
	report := flag.Bool("report", false, "probe everything and send a status report, then exit")
	flag.Parse()

	log.SetFlags(0)

	if *testNotify {
		if err := notify.NewDiscord(os.Getenv("DISCORD_WEBHOOK")).SendTest(); err != nil {
			log.Fatalf("test notification failed: %v", err)
		}
		log.Println("test message sent")
		return
	}

	cipher, err := crypto.New(os.Getenv("STATE_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	if cipher.Enabled {
		log.Println("state encryption: on")
	}

	watches, err := config.Load(*configPath, cipher)
	if err != nil {
		log.Fatal(err)
	}

	if *list {
		fmt.Println("configured watches:")
		for _, w := range watches {
			fmt.Printf("  - %s (%s)\n", w.ID, w.Type)
		}
		return
	}

	notifier := notify.NewDiscord(os.Getenv("DISCORD_WEBHOOK"))
	if !notifier.Enabled() {
		log.Println("warning: DISCORD_WEBHOOK not set — logging only")
	}

	store := state.New(*stateDir, cipher)
	eng := engine.New(watches, store, notifier)

	if *report {
		summary, err := eng.Report(context.Background())
		if err != nil {
			log.Fatalf("report failed: %v", err)
		}
		if err := notifier.SendReport(summary); err != nil {
			log.Fatalf("report send failed: %v", err)
		}
		log.Println("report sent")
		return
	}

	changed, err := eng.Run(context.Background())
	if err != nil {
		log.Fatalf("run failed: %v", err)
	}
	log.Printf("done (state changed: %t)", changed)
}
