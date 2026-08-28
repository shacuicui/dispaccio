package check

import (
	"encoding/json"
	"fmt"
)

type HTTPStatusCheck struct {
	label string
	url   string
}

type httpStatusConfig struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func NewHTTPStatusCheck(label string, raw json.RawMessage) (Check, error) {
	var c httpStatusConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("http_status: invalid config: %w", err)
	}
	if c.URL == "" {
		return nil, fmt.Errorf("http_status: url is required")
	}
	if c.Label != "" {
		label = c.Label
	}
	return &HTTPStatusCheck{label: label, url: c.URL}, nil
}

type httpStatusSnapshot struct {
	Status int `json:"status"`
}

func (h *HTTPStatusCheck) Snapshot() (Snapshot, error) {
	status, _ := fetch(h.url)
	return json.Marshal(httpStatusSnapshot{Status: status})
}

func (h *HTTPStatusCheck) Diff(old, current Snapshot) ([]Event, error) {
	if old == nil {
		return nil, nil
	}
	var prev, cur httpStatusSnapshot
	if err := json.Unmarshal(old, &prev); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(current, &cur); err != nil {
		return nil, err
	}
	if prev.Status == cur.Status {
		return nil, nil
	}

	wasUp := prev.Status != 0
	nowUp := cur.Status != 0
	switch {
	case !wasUp && nowUp:
		return []Event{{
			Level:       LevelUp,
			Title:       fmt.Sprintf("%s is now online", h.label),
			Description: "A dormant endpoint started answering.",
		}}, nil
	case wasUp && !nowUp:
		return []Event{{
			Level: LevelDown,
			Title: fmt.Sprintf("%s went offline", h.label),
		}}, nil
	default:
		return []Event{{
			Level: LevelChange,
			Title: fmt.Sprintf("%s status changed", h.label),
		}}, nil
	}
}

func (h *HTTPStatusCheck) Report(current Snapshot) string {
	var snap httpStatusSnapshot
	if err := json.Unmarshal(current, &snap); err != nil {
		return fmt.Sprintf("%s: unreadable", h.label)
	}
	if snap.Status != 0 {
		return fmt.Sprintf("%s: online", h.label)
	}
	return fmt.Sprintf("%s: offline", h.label)
}
