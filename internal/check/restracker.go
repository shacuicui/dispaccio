package check

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type ResTrackerCheck struct {
	label    string
	baseURL  string
	version  string
	channel  int
	platform int
	lang     int
	seed     string
	extra    string
}

type resTrackerConfig struct {
	Label    string `json:"label"`
	BaseURL  string `json:"base_url"`
	Version  string `json:"version"`
	Channel  int    `json:"channel_id"`
	Platform int    `json:"platform"`
	Lang     int    `json:"lang"`
	Seed     string `json:"seed"`
	Extra    string `json:"extra"`
}

func NewResTrackerCheck(label string, raw json.RawMessage) (Check, error) {
	var c resTrackerConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("res_tracker: invalid config: %w", err)
	}
	if c.BaseURL == "" || c.Version == "" {
		return nil, fmt.Errorf("res_tracker: base_url and version are required")
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
	return &ResTrackerCheck{
		label:    label,
		baseURL:  c.BaseURL,
		version:  c.Version,
		channel:  c.Channel,
		platform: c.Platform,
		lang:     c.Lang,
		seed:     c.Seed,
		extra:    c.Extra,
	}, nil
}

type resBuild struct {
	OK                bool   `json:"ok"`
	Branch            string `json:"branch"`
	ResVersion        int64  `json:"res_version"`
	Suffix            string `json:"suffix"`
	SilenceResVersion int64  `json:"silence_res_version"`
	SilenceSuffix     string `json:"silence_suffix"`
	CDNBase           string `json:"cdn_base"`
	IndexAndroid      string `json:"index_android"`
	IndexPC           string `json:"index_pc"`
	IndexData         string `json:"index_data"`
	IndexSilence      string `json:"index_silence"`
}

func (r *ResTrackerCheck) url() string {
	u := fmt.Sprintf(
		"%s?version=%s&channel_id=%d&platform=%d&lang=%d&dispatchSeed=%s",
		r.baseURL, r.version, r.channel, r.platform, r.lang, r.seed,
	)
	if r.extra != "" {
		u += "&" + r.extra
	}
	return u
}

func (r *ResTrackerCheck) Snapshot() (Snapshot, error) {
	status, body := fetch(r.url())
	var b resBuild
	if status == 200 && len(body) > 0 {
		if raw, err := base64.StdEncoding.DecodeString(string(body)); err == nil {
			parseResBuild(raw, &b)
		}
	}
	if b.Branch != "" && b.ResVersion != 0 && b.CDNBase != "" {
		b.OK = true
		base := strings.TrimRight(b.CDNBase, "/")
		out := fmt.Sprintf("output_%d_%s", b.ResVersion, b.Suffix)
		r.buildLinks(&b, base, out)
	}
	return json.Marshal(b)
}

func (r *ResTrackerCheck) buildLinks(b *resBuild, base, out string) {
	b.IndexAndroid = fmt.Sprintf("%s/game_res/%s/%s/client/Android/res_versions.json", base, b.Branch, out)
	b.IndexPC = fmt.Sprintf("%s/game_res/%s/%s/client/StandaloneWindows64/res_versions.json", base, b.Branch, out)
	b.IndexData = fmt.Sprintf("%s/design_data/%s/%s/client/General/data_versions.json", base, b.Branch, out)
	if b.SilenceResVersion != 0 {
		sout := fmt.Sprintf("output_%d_%s", b.SilenceResVersion, b.SilenceSuffix)
		b.IndexSilence = fmt.Sprintf("%s/design_data/%s/%s/client_silence/General/silence_data_versions.json", base, b.Branch, sout)
	}
}

func (r *ResTrackerCheck) Diff(old, current Snapshot) ([]Event, error) {
	if old == nil {
		return nil, nil
	}
	var prev, cur resBuild
	if err := json.Unmarshal(old, &prev); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(current, &cur); err != nil {
		return nil, err
	}
	if !cur.OK {
		return nil, nil
	}
	if cur.ResVersion == prev.ResVersion && cur.Suffix == prev.Suffix &&
		cur.SilenceResVersion == prev.SilenceResVersion {
		return nil, nil
	}

	title := fmt.Sprintf("%s: NEW resource build %s (res_version %d)", r.label, cur.Branch, cur.ResVersion)
	var desc strings.Builder
	fmt.Fprintf(&desc, "branch: %s\nres_version: %d\nsuffix: %s\n", cur.Branch, cur.ResVersion, cur.Suffix)
	if cur.SilenceResVersion != 0 {
		fmt.Fprintf(&desc, "silence res_version: %d\nsilence suffix: %s\n", cur.SilenceResVersion, cur.SilenceSuffix)
	}
	desc.WriteString("\nIndices:\n")
	fmt.Fprintf(&desc, "Android: %s\n", cur.IndexAndroid)
	fmt.Fprintf(&desc, "PC: %s\n", cur.IndexPC)
	fmt.Fprintf(&desc, "data: %s\n", cur.IndexData)
	if cur.IndexSilence != "" {
		fmt.Fprintf(&desc, "silence: %s\n", cur.IndexSilence)
	}

	return []Event{{
		Level:       LevelNew,
		Title:       title,
		Description: desc.String(),
		Fields: map[string]string{
			"branch":      cur.Branch,
			"res_version": fmt.Sprintf("%d", cur.ResVersion),
			"suffix":      cur.Suffix,
		},
	}}, nil
}

func FetchURL(url string) []byte {
	status, body := fetch(url)
	if status != 200 {
		return nil
	}
	return body
}

type ArchiveTarget struct {
	RelPath string
	URL     string
}

func ArchiveTargets(snap Snapshot) []ArchiveTarget {
	var b resBuild
	if err := json.Unmarshal(snap, &b); err != nil || !b.OK {
		return nil
	}
	dir := fmt.Sprintf("builds/%s_%d", b.Branch, b.ResVersion)
	targets := []ArchiveTarget{
		{RelPath: dir + "/android.dat", URL: b.IndexAndroid},
		{RelPath: dir + "/pc.dat", URL: b.IndexPC},
		{RelPath: dir + "/data.dat", URL: b.IndexData},
	}
	if b.IndexSilence != "" {
		targets = append(targets, ArchiveTarget{RelPath: dir + "/silence.dat", URL: b.IndexSilence})
	}
	return targets
}

func (r *ResTrackerCheck) Report(current Snapshot) string {
	var b resBuild
	if err := json.Unmarshal(current, &b); err != nil {
		return fmt.Sprintf("%s: unreadable", r.label)
	}
	if !b.OK {
		return fmt.Sprintf("%s: no build", r.label)
	}
	return fmt.Sprintf("%s → %s res_version %d (suffix %s)", r.label, b.Branch, b.ResVersion, b.Suffix)
}

func readTag(buf []byte, i int) (field, wire, next int) {
	v, n := readVarint(buf, i)
	return int(v >> 3), int(v & 0x07), n
}

func scan(buf []byte, fn func(field int, val []byte)) {
	i := 0
	for i < len(buf) {
		field, wire, n := readTag(buf, i)
		i = n
		switch wire {
		case wireVarint:
			_, i = readVarint(buf, i)
		case wireBytes:
			var length uint64
			length, i = readVarint(buf, i)
			end := i + int(length)
			if end > len(buf) || end < i {
				return
			}
			fn(field, buf[i:end])
			i = end
		case 5: // fixed32
			i += 4
		case 1: // fixed64
			i += 8
		default:
			return
		}
	}
}

func parseResBuild(raw []byte, b *resBuild) {
	inner := findField(raw, 3)
	if inner == nil {
		inner = raw
	}
	scan(inner, func(field int, val []byte) {
		switch field {
		case 8, 9:
			if b.CDNBase == "" {
				b.CDNBase = string(val)
			}
		case 22:
			rv, sfx, branch := parseBuildBlock(val)
			if rv != 0 {
				b.SilenceResVersion = rv
			}
			if sfx != "" {
				b.SilenceSuffix = sfx
			}
			if b.Branch == "" && branch != "" {
				b.Branch = branch
			}
		case 36:
			rv, sfx, branch := parseBuildBlock(val)
			if rv != 0 {
				b.ResVersion = rv
			}
			if sfx != "" {
				b.Suffix = sfx
			}
			if branch != "" {
				b.Branch = branch
			}
		}
	})
	if b.ResVersion == 0 && b.SilenceResVersion != 0 {
		b.ResVersion = b.SilenceResVersion
		b.Suffix = b.SilenceSuffix
	}
}

func parseBuildBlock(sub []byte) (resVersion int64, suffix, branch string) {
	i := 0
	for i < len(sub) {
		field, wire, n := readTag(sub, i)
		i = n
		switch wire {
		case wireVarint:
			var v uint64
			v, i = readVarint(sub, i)
			if field == 1 && resVersion == 0 {
				resVersion = int64(v)
			}
		case wireBytes:
			var length uint64
			length, i = readVarint(sub, i)
			end := i + int(length)
			if end > len(sub) || end < i {
				return
			}
			s := string(sub[i:end])
			i = end
			if strings.HasPrefix(s, "live_") {
				branch = s
			} else if suffix == "" && isHexSuffix(s) {
				suffix = s
			}
		case 5:
			i += 4
		case 1:
			i += 8
		default:
			return
		}
	}
	return resVersion, suffix, branch
}

func isHexSuffix(s string) bool {
	if len(s) < 6 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func findField(buf []byte, want int) []byte {
	var out []byte
	scan(buf, func(field int, val []byte) {
		if field == want && out == nil {
			out = val
		}
	})
	return out
}
