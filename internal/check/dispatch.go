package check

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type DispatchCheck struct {
	label    string
	baseURL  string
	versions []string
	channel  int
	platform int
	lang     int
	seed     string
}

type dispatchConfig struct {
	Label    string   `json:"label"`
	BaseURL  string   `json:"base_url"`
	Versions []string `json:"versions"`
	Channel  int      `json:"channel_id"`
	Platform int      `json:"platform"`
	Lang     int      `json:"lang"`
	Seed     string   `json:"seed"`
}

func NewDispatchCheck(label string, raw json.RawMessage) (Check, error) {
	var c dispatchConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("dispatch: invalid config: %w", err)
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("dispatch: base_url is required")
	}
	if c.Channel == 0 {
		c.Channel = 1
	}
	if c.Lang == 0 {
		c.Lang = 15
	}
	if c.Seed == "" {
		c.Seed = "00000000"
	}
	if c.Label != "" {
		label = c.Label
	}
	return &DispatchCheck{
		label:    label,
		baseURL:  c.BaseURL,
		versions: c.Versions,
		channel:  c.Channel,
		platform: c.Platform,
		lang:     c.Lang,
		seed:     c.Seed,
	}, nil
}

type region struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

type versionResult struct {
	Status     int      `json:"status"`
	Retcode    *int64   `json:"retcode"`
	Hash       string   `json:"hash"`
	Configured bool     `json:"configured"`
	Regions    []region `json:"regions"`
}

type dispatchSnapshot map[string]versionResult

func (d *DispatchCheck) url(version string) string {
	return fmt.Sprintf(
		"%s?version=%s&channel_id=%d&platform=%d&lang=%d&dispatchSeed=%s",
		d.baseURL, version, d.channel, d.platform, d.lang, d.seed,
	)
}

func (d *DispatchCheck) Snapshot() (Snapshot, error) {
	snap := make(dispatchSnapshot, len(d.versions))
	for _, v := range d.versions {
		status, body := fetch(d.url(v))
		res := versionResult{Status: status}

		var raw []byte
		if status == 200 && len(body) > 0 {
			if decoded, err := base64.StdEncoding.DecodeString(string(body)); err == nil {
				raw = decoded
			}
		}
		if len(raw) > 0 {
			sum := sha1.Sum(raw)
			res.Hash = hex.EncodeToString(sum[:])
			if field, wire := tag(raw[0]); field == 2 && wire == wireBytes {
				res.Configured = true
				res.Regions = parseRegions(raw)
			} else if rc, ok := readRetcode(raw); ok {
				res.Retcode = &rc
			}
		}
		snap[v] = res
	}
	return json.Marshal(snap)
}

func (d *DispatchCheck) Diff(old, current Snapshot) ([]Event, error) {
	if old == nil {
		return nil, nil
	}
	var prev, cur dispatchSnapshot
	if err := json.Unmarshal(old, &prev); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(current, &cur); err != nil {
		return nil, err
	}

	var events []Event
	for version, now := range cur {
		before, ok := prev[version]
		if !ok {
			continue
		}
		switch {
		case now.Configured && !before.Configured:
			events = append(events, Event{
				Level:       LevelNew,
				Title:       fmt.Sprintf("%s: region list for %s", d.label, version),
				Description: describeRegions(now.Regions),
				Fields: map[string]string{
					"version": version,
					"regions": fmt.Sprintf("%d", len(now.Regions)),
				},
			})
		case retcodeChanged(before.Retcode, now.Retcode):
			events = append(events, Event{
				Level:  LevelChange,
				Title:  fmt.Sprintf("%s: retcode changed for %s", d.label, version),
				Fields: map[string]string{"version": version},
			})
		}
	}
	return events, nil
}

func readRetcode(raw []byte) (int64, bool) {
	if len(raw) == 0 || raw[0] != 0x08 {
		return 0, false
	}
	value, _ := readVarint(raw, 1)
	return int64(value), true
}

func parseRegions(raw []byte) []region {
	var regions []region
	i := 0
	for i < len(raw) {
		field, wire := tag(raw[i])
		i++
		switch {
		case field == 1 && wire == wireVarint:
			_, i = readVarint(raw, i)
		case field == 2 && wire == wireBytes:
			var length uint64
			length, i = readVarint(raw, i)
			end := i + int(length)
			if end > len(raw) {
				end = len(raw)
			}
			regions = append(regions, parseRegionInfo(raw[i:end]))
			i = end
		default:
			if wire == wireBytes {
				var length uint64
				length, i = readVarint(raw, i)
				i += int(length)
			} else {
				return regions
			}
		}
	}
	return regions
}

func parseRegionInfo(sub []byte) region {
	var r region
	i := 0
	for i < len(sub) {
		field, wire := tag(sub[i])
		i++
		if wire == wireVarint {
			_, i = readVarint(sub, i)
			continue
		}
		if wire != wireBytes {
			break
		}
		var length uint64
		length, i = readVarint(sub, i)
		end := i + int(length)
		if end > len(sub) {
			end = len(sub)
		}
		value := string(sub[i:end])
		i = end
		switch field {
		case 1:
			r.Name = value
		case 2:
			r.Title = value
		case 3:
			r.Type = value
		case 4:
			r.URL = value
		}
	}
	return r
}

func describeRegions(regions []region) string {
	if len(regions) == 0 {
		return "A real server list is now being served."
	}
	lines := make([]string, 0, len(regions)+1)
	lines = append(lines, "A real server list is now being served:")
	for _, r := range regions {
		name := r.Name
		if name == "" {
			name = "?"
		}
		lines = append(lines, fmt.Sprintf("%s (%s): %s", name, r.Type, r.URL))
	}
	return strings.Join(lines, "\n")
}

func retcodeChanged(a, b *int64) bool {
	switch {
	case a == nil && b == nil:
		return false
	case a == nil || b == nil:
		return true
	default:
		return *a != *b
	}
}

func (d *DispatchCheck) Report(current Snapshot) string {
	var snap dispatchSnapshot
	if err := json.Unmarshal(current, &snap); err != nil {
		return fmt.Sprintf("%s: unreadable", d.label)
	}
	var live []string
	for version, res := range snap {
		if res.Configured {
			names := make([]string, 0, len(res.Regions))
			for _, r := range res.Regions {
				names = append(names, r.Name)
			}
			live = append(live, fmt.Sprintf("%s → %s", version, strings.Join(names, ", ")))
		}
	}
	if len(live) == 0 {
		return fmt.Sprintf("%s: no region list", d.label)
	}
	return fmt.Sprintf("%s: %s", d.label, strings.Join(live, "; "))
}
