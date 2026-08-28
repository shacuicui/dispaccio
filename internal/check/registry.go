package check

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Factory func(label string, raw json.RawMessage) (Check, error)

var registry = map[string]Factory{
	"http_status": NewHTTPStatusCheck,
	"dispatch":    NewDispatchCheck,
	"launcher":    NewLauncherCheck,
}

func Build(typeName, label string, raw json.RawMessage) (Check, error) {
	factory, ok := registry[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown check type %q (known: %v)", typeName, KnownTypes())
	}
	return factory(label, raw)
}

func KnownTypes() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
