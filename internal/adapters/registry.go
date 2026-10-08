package adapters

import (
	"fmt"
	"sort"
	"strings"
)

type Registry struct {
	adapters map[string]ClientAdapter
}

func NewRegistry(initial ...ClientAdapter) *Registry {
	registry := &Registry{adapters: make(map[string]ClientAdapter, len(initial))}
	for _, adapter := range initial {
		if err := registry.Register(adapter); err != nil {
			panic(err)
		}
	}
	return registry
}

func (r *Registry) Register(adapter ClientAdapter) error {
	if adapter == nil {
		return fmt.Errorf("adapter is nil")
	}
	id := strings.TrimSpace(adapter.ID())
	if id == "" {
		return fmt.Errorf("adapter ID is empty")
	}
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("adapter %q is already registered", id)
	}
	r.adapters[id] = adapter
	return nil
}

func (r *Registry) Select(names []string) ([]ClientAdapter, error) {
	if len(names) == 0 || (len(names) == 1 && names[0] == "all") {
		ids := make([]string, 0, len(r.adapters))
		for id := range r.adapters {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		selected := make([]ClientAdapter, 0, len(ids))
		for _, id := range ids {
			selected = append(selected, r.adapters[id])
		}
		return selected, nil
	}

	seen := make(map[string]struct{}, len(names))
	selected := make([]ClientAdapter, 0, len(names))
	for _, name := range names {
		if name == "all" {
			return nil, fmt.Errorf("client %q cannot be combined with named clients", name)
		}
		adapter, exists := r.adapters[name]
		if !exists {
			return nil, fmt.Errorf("unknown client %q", name)
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		selected = append(selected, adapter)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID() < selected[j].ID() })
	return selected, nil
}
