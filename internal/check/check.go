package check

import "encoding/json"

type Level string

const (
	LevelUp     Level = "up"
	LevelDown   Level = "down"
	LevelNew    Level = "new"
	LevelChange Level = "change"
)

type Event struct {
	Level       Level
	Title       string
	Description string
	Fields      map[string]string
}

type Snapshot = json.RawMessage

type Check interface {
	Snapshot() (Snapshot, error)
	Diff(old, current Snapshot) ([]Event, error)
	Report(current Snapshot) string
}
