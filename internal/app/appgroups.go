package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"

	"ferry/internal/spec"
)

type AppGroups struct{ session *Session }

func (s *Session) AppGroups() *AppGroups { return &AppGroups{s} }

func (a *AppGroups) List(context.Context) ([]spec.AppGroup, error) {
	a.session.mu.Lock()
	defer a.session.mu.Unlock()
	return cloneAppGroups(a.session.config.AppGroups), nil
}

func cloneAppGroup(group spec.AppGroup) spec.AppGroup {
	group.Apps = append([]string{}, group.Apps...)
	return group
}

func cloneAppGroups(groups []spec.AppGroup) []spec.AppGroup {
	result := make([]spec.AppGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, cloneAppGroup(group))
	}
	return result
}

func (a *AppGroups) Save(ctx context.Context, group spec.AppGroup) (spec.AppGroup, error) {
	group = cloneAppGroup(group)
	if group.ID == "" {
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			return spec.AppGroup{}, errors.New("无法创建应用分组标识")
		}
		group.ID = "ag-" + hex.EncodeToString(id[:])
	}
	err := a.mutate(ctx, func(config *spec.RunSpec) error {
		for i, current := range config.AppGroups {
			if current.ID == group.ID {
				config.AppGroups[i] = cloneAppGroup(group)
				return nil
			}
		}
		config.AppGroups = append(config.AppGroups, cloneAppGroup(group))
		return nil
	})
	if err != nil {
		return spec.AppGroup{}, err
	}
	return group, nil
}

func (a *AppGroups) Remove(ctx context.Context, id string) error {
	return a.mutate(ctx, func(config *spec.RunSpec) error {
		index := slices.IndexFunc(config.AppGroups, func(group spec.AppGroup) bool { return group.ID == id })
		if index < 0 {
			return errors.New("应用分组不存在")
		}
		for _, rule := range config.Rules {
			if slices.Contains(rule.Apps, spec.AppGroupPrefix+id) {
				return errors.New("规则仍在引用此应用分组，请先移除对应引用")
			}
		}
		config.AppGroups = slices.Delete(config.AppGroups, index, index+1)
		return nil
	})
}

func (a *AppGroups) mutate(ctx context.Context, change func(*spec.RunSpec) error) error {
	a.session.mu.Lock()
	next := a.session.snapshot()
	next.Config.AppGroups = cloneAppGroups(next.Config.AppGroups)
	err := change(&next.Config)
	if err == nil {
		err = next.Config.Validate()
	}
	if err == nil {
		err = validateImportedConfig(next.Config)
	}
	if err == nil && a.session.reconfigure(ctx, next) != nil {
		err = errors.New("应用分组未能应用，请检查内核状态与已保存配置")
	}
	version := a.session.revision
	a.session.mu.Unlock()
	if err == nil {
		a.session.emit("rules:applied", map[string]uint64{"version": version})
	}
	return err
}
