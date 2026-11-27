package app

import (
	"ferry/internal/spec"
	"slices"
)

func mergeImportedGroups(config *spec.RunSpec, incoming []spec.Group) []string {
	config.Groups = slices.Clone(config.Groups)
	known := map[string]bool{}
	for _, group := range config.Groups {
		known[group.ID] = true
	}
	owned := []string{}
	for _, group := range incoming {
		if !known[group.ID] {
			config.Groups = append(config.Groups, group)
			owned = append(owned, group.ID)
			known[group.ID] = true
		}
	}
	return owned
}
