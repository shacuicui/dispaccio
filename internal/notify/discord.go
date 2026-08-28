package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shacuicui/dispaccio/internal/check"
)

var colours = map[check.Level]int{
	check.LevelUp:     0x2ECC71, // green
	check.LevelDown:   0xE74C3C, // red
	check.LevelNew:    0x3498DB, // blue
	check.LevelChange: 0xF1C40F, // yellow
}

const defaultColour = 0x95A5A6 // grey

type Discord struct {
	webhookURL string
	client     *http.Client
}

func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 12 * time.Second},
	}
}

func (d *Discord) Enabled() bool { return d.webhookURL != "" }

type embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields,omitempty"`
	Footer      *embedFooter `json:"footer,omitempty"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type embedFooter struct {
	Text string `json:"text"`
}

func (d *Discord) Send(watchID string, events []check.Event) error {
	if !d.Enabled() || len(events) == 0 {
		return nil
	}

	embeds := make([]embed, 0, len(events))
	for _, ev := range events {
		e := embed{
			Title:  ev.Title,
			Color:  colourFor(ev.Level),
			Footer: &embedFooter{Text: "watch: " + watchID},
		}
		if ev.Description != "" {
			e.Description = truncate(ev.Description, 4000)
		}
		for k, v := range ev.Fields {
			e.Fields = append(e.Fields, embedField{Name: k, Value: v, Inline: true})
		}
		embeds = append(embeds, e)
	}

	for i := 0; i < len(embeds); i += 10 {
		end := i + 10
		if end > len(embeds) {
			end = len(embeds)
		}
		if err := d.post(embeds[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discord) SendTest() error {
	if !d.Enabled() {
		return fmt.Errorf("no webhook configured")
	}
	return d.post([]embed{{
		Title:       "dispaccio test",
		Description: "If you can read this, the webhook works.",
		Color:       colours[check.LevelUp],
	}})
}

func (d *Discord) post(embeds []embed) error {
	body, err := json.Marshal(map[string]any{"embeds": embeds})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "dispaccio/1.0 (+https://github.com)")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}

func colourFor(l check.Level) int {
	if c, ok := colours[l]; ok {
		return c
	}
	return defaultColour
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func (d *Discord) SendReport(summary string) error {
	if !d.Enabled() {
		return fmt.Errorf("no webhook configured")
	}
	const maxLen = 1900
	lines := strings.Split(summary, "\n")

	var buf strings.Builder
	flush := func() error {
		if buf.Len() == 0 {
			return nil
		}
		err := d.postContent(buf.String())
		buf.Reset()
		return err
	}

	for _, line := range lines {
		if buf.Len()+len(line)+1 > maxLen {
			if err := flush(); err != nil {
				return err
			}
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	return flush()
}

func (d *Discord) postContent(content string) error {
	body, err := json.Marshal(map[string]any{"content": content})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "dispaccio/1.0 (+https://github.com)")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}
