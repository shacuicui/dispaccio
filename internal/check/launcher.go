package check

import (
	"encoding/json"
	"fmt"
)

type LauncherCheck struct {
	label        string
	biz          string
	packagesURL  string
	basicInfoURL string
}

type launcherConfig struct {
	Label        string `json:"label"`
	Biz          string `json:"biz"`
	PackagesURL  string `json:"packages_url"`
	BasicInfoURL string `json:"basic_info_url"`
}

func NewLauncherCheck(label string, raw json.RawMessage) (Check, error) {
	var c launcherConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("launcher: invalid config: %w", err)
	}
	if c.Biz == "" || c.PackagesURL == "" || c.BasicInfoURL == "" {
		return nil, fmt.Errorf("launcher: biz, packages_url and basic_info_url are required")
	}
	if c.Label != "" {
		label = c.Label
	}
	return &LauncherCheck{
		label:        label,
		biz:          c.Biz,
		packagesURL:  c.PackagesURL,
		basicInfoURL: c.BasicInfoURL,
	}, nil
}

type launcherSnapshot struct {
	Present            bool     `json:"present"`
	Version            string   `json:"version"`
	PreDownloadVersion string   `json:"predownload_version"`
	Videos             []string `json:"videos"`
	Images             []string `json:"images"`
}

func (l *LauncherCheck) Snapshot() (Snapshot, error) {
	snap := launcherSnapshot{}

	if pkgs := fetchJSON(l.packagesURL); pkgs != nil {
		for _, g := range digList(pkgs, "data", "game_packages") {
			if digString(g, "game", "biz") != l.biz {
				continue
			}
			snap.Present = true
			snap.Version = digString(g, "main", "major", "version")
			snap.PreDownloadVersion = digString(g, "pre_download", "major", "version")
			break
		}
	}

	if info := fetchJSON(l.basicInfoURL); info != nil {
		for _, g := range digList(info, "data", "game_info_list") {
			if digString(g, "game", "biz") != l.biz {
				continue
			}
			for _, bg := range digList(g, "backgrounds") {
				if v := digString(bg, "video", "url"); v != "" {
					snap.Videos = append(snap.Videos, v)
				}
				if img := digString(bg, "background", "url"); img != "" {
					snap.Images = append(snap.Images, img)
				}
			}
			break
		}
	}

	snap.Videos = dedupe(snap.Videos)
	snap.Images = dedupe(snap.Images)
	return json.Marshal(snap)
}

func (l *LauncherCheck) Diff(old, current Snapshot) ([]Event, error) {
	if old == nil {
		return nil, nil
	}
	var prev, cur launcherSnapshot
	if err := json.Unmarshal(old, &prev); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(current, &cur); err != nil {
		return nil, err
	}

	var events []Event

	if cur.Present && !prev.Present {
		events = append(events, Event{
			Level:       LevelNew,
			Title:       fmt.Sprintf("%s: appeared in the launcher", l.label),
			Description: "The product now appears in the launcher API — usually the earliest strong sign of a public release.",
			Fields:      map[string]string{"version": cur.Version},
		})
	}
	if cur.Version != "" && cur.Version != prev.Version {
		events = append(events, Event{
			Level:  LevelNew,
			Title:  fmt.Sprintf("%s: new build %s", l.label, cur.Version),
			Fields: map[string]string{"version": cur.Version},
		})
	}
	if cur.PreDownloadVersion != "" && cur.PreDownloadVersion != prev.PreDownloadVersion {
		events = append(events, Event{
			Level: LevelNew,
			Title: fmt.Sprintf("%s: pre-download opened (%s)", l.label, cur.PreDownloadVersion),
		})
	}
	for _, url := range cur.Videos {
		if !contains(prev.Videos, url) {
			events = append(events, Event{
				Level:       LevelNew,
				Title:       fmt.Sprintf("%s: new background video", l.label),
				Description: url,
			})
		}
	}
	for _, url := range cur.Images {
		if !contains(prev.Images, url) {
			events = append(events, Event{
				Level:       LevelNew,
				Title:       fmt.Sprintf("%s: new background image", l.label),
				Description: url,
			})
		}
	}
	return events, nil
}

func (l *LauncherCheck) Report(current Snapshot) string {
	var snap launcherSnapshot
	if err := json.Unmarshal(current, &snap); err != nil {
		return fmt.Sprintf("%s: unreadable", l.label)
	}
	if !snap.Present {
		return fmt.Sprintf("%s: not present", l.label)
	}
	return fmt.Sprintf("%s: present, version %s, %d video(s), %d image(s)",
		l.label, snap.Version, len(snap.Videos), len(snap.Images))
}
