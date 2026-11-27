package spec

import (
	"errors"
	"strings"
)

const AppGroupPrefix = "@group:"

func (s RunSpec) validateAppGroups() error {
	if len(s.AppGroups) > MaxAppGroups {
		return errors.New("应用分组数量超过限制")
	}
	seen := make(map[string]bool, len(s.AppGroups))
	for _, group := range s.AppGroups {
		if seen[group.ID] {
			return errors.New("应用分组标识重复")
		}
		seen[group.ID] = true
		if err := validateAppGroup(group); err != nil {
			return err
		}
	}
	return nil
}

func validateAppGroup(group AppGroup) error {
	if !validID(group.ID) || !validLabel(group.Name) {
		return errors.New("应用分组标识或名称无效")
	}
	if len(group.Apps) > MaxRuleItems {
		return errors.New("应用分组成员数量超过限制")
	}
	for _, app := range group.Apps {
		if strings.HasPrefix(app, "@") || !validRuleApp(app) {
			return errors.New("应用分组成员无效，不能引用其它分组")
		}
	}
	return nil
}

// ExpandApps validates and expands group references for process matching only.
// Members retain rule OR semantics; duplicates cannot multiply compiled patterns.
func (s RunSpec) ExpandApps(apps []string) ([]string, error) {
	if len(apps) > MaxRuleItems || len(s.AppGroups) > MaxAppGroups {
		return nil, errors.New("应用条件或分组数量超过限制")
	}
	result := make([]string, 0, len(apps))
	seen := make(map[string]bool)
	for _, app := range apps {
		members := []string{app}
		if strings.HasPrefix(app, "@") {
			var err error
			members, err = s.appGroupMembers(app)
			if err != nil {
				return nil, err
			}
		} else if !validRuleApp(app) {
			return nil, errors.New("规则应用条件无效")
		}
		for _, member := range members {
			if !seen[member] {
				if len(result) >= MaxExpandedRuleApps {
					return nil, errors.New("单条规则展开后的应用条件超过限制")
				}
				result = append(result, member)
				seen[member] = true
			}
		}
	}
	return result, nil
}

func (s RunSpec) validateAppExpansion() error {
	total := 0
	for _, rule := range s.Rules {
		apps, err := s.ExpandApps(rule.Apps)
		if err != nil {
			return err
		}
		total += len(apps)
		if total > MaxTotalRuleApps {
			return errors.New("所有规则展开后的应用条件总数超过限制")
		}
	}
	return nil
}

func (s RunSpec) appGroupMembers(reference string) ([]string, error) {
	id, found := strings.CutPrefix(reference, AppGroupPrefix)
	if !found || !validID(id) {
		return nil, errors.New("应用分组引用格式无效")
	}
	var members []string
	for _, group := range s.AppGroups {
		if group.ID != id {
			continue
		}
		if members != nil || len(group.Apps) == 0 || validateAppGroup(group) != nil {
			return nil, errors.New("引用的应用分组为空或无效")
		}
		members = group.Apps
	}
	if members == nil {
		return nil, errors.New("引用的应用分组不存在")
	}
	return members, nil
}
