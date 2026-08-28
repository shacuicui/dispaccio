package notify

import "github.com/shacuicui/dispaccio/internal/check"

type Notifier interface {
	Send(watchID string, events []check.Event) error
}
