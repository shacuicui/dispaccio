package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/shacuicui/dispaccio/internal/check"
	"github.com/shacuicui/dispaccio/internal/config"
	"github.com/shacuicui/dispaccio/internal/notify"
	"github.com/shacuicui/dispaccio/internal/state"
)

const maxConcurrent = 16

type Engine struct {
	watches  []config.Watch
	store    *state.Store
	notifier notify.Notifier
}

func New(watches []config.Watch, store *state.Store, notifier notify.Notifier) *Engine {
	return &Engine{watches: watches, store: store, notifier: notifier}
}

type result struct {
	watch    config.Watch
	chk      check.Check
	snapshot check.Snapshot
	err      error
}

func tag(watchID string) string {
	sum := sha256.Sum256([]byte(watchID))
	return hex.EncodeToString(sum[:4])
}

func (e *Engine) Run(ctx context.Context) (bool, error) {
	log.Printf("loaded %d watch(es)", len(e.watches))

	results := make([]result, len(e.watches))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i, w := range e.watches {
		i, w := i, w
		g.Go(func() error {
			chk, err := check.Build(w.Type, w.Label, w.Config)
			if err != nil {
				results[i] = result{watch: w, err: err}
				return nil
			}
			snap, err := chk.Snapshot()
			results[i] = result{watch: w, chk: chk, snapshot: snap, err: err}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return false, err
	}

	changed := false
	for _, r := range results {
		id := tag(r.watch.ID)
		if r.err != nil {
			log.Printf("%s: error", id)
			continue
		}

		old, err := e.store.Load(r.watch.ID)
		if err != nil {
			log.Printf("%s: load state failed", id)
			continue
		}

		events, err := r.chk.Diff(old, r.snapshot)
		if err != nil {
			log.Printf("%s: diff failed", id)
			continue
		}

		switch {
		case old == nil:
			log.Printf("%s: baseline stored", id)
		case len(events) > 0:
			log.Printf("%s: %d event(s)", id, len(events))
			if err := e.notifier.Send(r.watch.ID, events); err != nil {
				log.Printf("%s: notify failed", id)
			}
		default:
			log.Printf("%s: no change", id)
		}

		if !equalSnapshot(old, r.snapshot) {
			if err := e.store.Save(r.watch.ID, r.snapshot); err != nil {
				log.Printf("%s: save state failed", id)
				continue
			}
			changed = true
		}
	}
	return changed, nil
}

func (e *Engine) Report(ctx context.Context) (string, error) {
	log.Printf("report: summarising %d stored watch(es)", len(e.watches))

	var active []string
	var idle []string
	var offline []string
	var launcher []string
	var missing []string

	for _, w := range e.watches {
		chk, err := check.Build(w.Type, w.Label, w.Config)
		if err != nil {
			missing = append(missing, w.Label+" (config error)")
			continue
		}
		snap, err := e.store.Load(w.ID)
		if err != nil || snap == nil {
			missing = append(missing, w.Label)
			continue
		}
		line := chk.Report(snap)

		switch {
		case w.Type == "launcher":
			launcher = append(launcher, line)
		case strings.Contains(line, "→"):
			active = append(active, line)
		case strings.HasSuffix(line, ": offline"):
			offline = append(offline, w.Label)
		case strings.HasSuffix(line, ": no region list"):
			idle = append(idle, w.Label)
		default:
			idle = append(idle, w.Label)
		}
	}

	var b strings.Builder
	b.WriteString("**Status report**\n\n")

	b.WriteString("🟢 **With region list**\n")
	if len(active) > 0 {
		for _, l := range active {
			b.WriteString(l)
			b.WriteString("\n")
		}
	} else {
		b.WriteString("(none)\n")
	}

	b.WriteString("\n📦 **Launcher**\n")
	if len(launcher) > 0 {
		for _, l := range launcher {
			b.WriteString(l)
			b.WriteString("\n")
		}
	} else {
		b.WriteString("(none)\n")
	}

	if len(idle) > 0 {
		b.WriteString("\n⚫ **Reachable, no region list** (")
		b.WriteString(itoa(len(idle)))
		b.WriteString(")\n")
		b.WriteString(strings.Join(idle, ", "))
		b.WriteString("\n")
	}
	if len(offline) > 0 {
		b.WriteString("\n🔴 **Offline** (")
		b.WriteString(itoa(len(offline)))
		b.WriteString(")\n")
		b.WriteString(strings.Join(offline, ", "))
		b.WriteString("\n")
	}
	if len(missing) > 0 {
		b.WriteString("\n❔ **No stored state** (")
		b.WriteString(itoa(len(missing)))
		b.WriteString(")\n")
		b.WriteString(strings.Join(missing, ", "))
		b.WriteString("\n")
	}

	return b.String(), nil
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func equalSnapshot(a, b check.Snapshot) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
