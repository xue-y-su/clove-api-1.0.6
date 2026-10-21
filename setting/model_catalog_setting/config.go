package model_catalog_setting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	OptionKey         = "model_catalog_setting.group_visibility"
	CurrentVersion    = 3
	ModeAll           = "all"
	ModeSelected      = "selected"
	DefaultModeAll    = "all"
	DefaultModeHidden = "hidden"
)

type GroupRule struct {
	Mode       string   `json:"mode"`
	UserGroups []string `json:"user_groups"`
}

type Visibility struct {
	Version     int                  `json:"version"`
	Enabled     bool                 `json:"enabled"`
	DefaultMode string               `json:"default_mode"`
	Rules       map[string]GroupRule `json:"rules"`
}

type visibilityV2 struct {
	Version     int                  `json:"version"`
	DefaultMode string               `json:"default_mode"`
	Rules       map[string]GroupRule `json:"rules"`
}

type legacyVisibility struct {
	Version int      `json:"version"`
	Mode    string   `json:"mode"`
	Groups  []string `json:"groups"`
}

func DefaultVisibility() Visibility {
	return Visibility{
		Version:     CurrentVersion,
		Enabled:     false,
		DefaultMode: DefaultModeAll,
		Rules:       map[string]GroupRule{},
	}
}

func EnabledVisibility() Visibility {
	return Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeAll,
		Rules:       map[string]GroupRule{},
	}
}

func HiddenByDefaultVisibility() Visibility {
	return Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: DefaultModeHidden,
		Rules:       map[string]GroupRule{},
	}
}

func ParseStored(raw string) (Visibility, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultVisibility(), nil
	}

	var fields map[string]json.RawMessage
	if err := common.UnmarshalJsonStr(raw, &fields); err != nil {
		if strings.Contains(err.Error(), "after top-level value") {
			return Visibility{}, fmt.Errorf("group visibility must contain exactly one JSON object")
		}
		return Visibility{}, fmt.Errorf("decode group visibility: %w", err)
	}

	var version int
	if encoded, exists := fields["version"]; exists {
		if err := common.Unmarshal(encoded, &version); err != nil {
			return Visibility{}, fmt.Errorf("decode group visibility version: %w", err)
		}
	}

	switch version {
	case 1:
		return parseLegacyVisibility(raw, fields)
	case 2:
		return parseV2Visibility(raw, fields)
	case CurrentVersion:
		return parseCurrentVisibility(raw, fields)
	default:
		return Visibility{}, fmt.Errorf("unsupported group visibility version %d", version)
	}
}

func parseLegacyVisibility(raw string, fields map[string]json.RawMessage) (Visibility, error) {
	for field := range fields {
		switch field {
		case "version", "mode", "groups":
		default:
			return Visibility{}, fmt.Errorf("decode group visibility: unknown field %q", field)
		}
	}

	var legacy legacyVisibility
	if err := common.UnmarshalJsonStr(raw, &legacy); err != nil {
		return Visibility{}, fmt.Errorf("decode group visibility: %w", err)
	}
	if legacy.Mode != ModeAll && legacy.Mode != ModeSelected {
		return Visibility{}, fmt.Errorf("unsupported group visibility mode %q", legacy.Mode)
	}
	groups, err := normalizeGroupNames(legacy.Groups)
	if err != nil {
		return Visibility{}, err
	}
	if legacy.Mode == ModeAll {
		return EnabledVisibility(), nil
	}

	converted := HiddenByDefaultVisibility()
	for _, group := range groups {
		converted.Rules[group] = GroupRule{Mode: ModeAll, UserGroups: []string{}}
	}
	return converted, nil
}

func parseV2Visibility(raw string, fields map[string]json.RawMessage) (Visibility, error) {
	if err := validateVisibilityFields(fields, false); err != nil {
		return Visibility{}, err
	}
	if err := validateRuleFields(fields); err != nil {
		return Visibility{}, err
	}

	var legacy visibilityV2
	if err := common.UnmarshalJsonStr(raw, &legacy); err != nil {
		return Visibility{}, fmt.Errorf("decode group visibility: %w", err)
	}
	return normalizeStructural(Visibility{
		Version:     CurrentVersion,
		Enabled:     true,
		DefaultMode: legacy.DefaultMode,
		Rules:       legacy.Rules,
	})
}

func parseCurrentVisibility(raw string, fields map[string]json.RawMessage) (Visibility, error) {
	if err := validateVisibilityFields(fields, true); err != nil {
		return Visibility{}, err
	}
	if err := validateRuleFields(fields); err != nil {
		return Visibility{}, err
	}

	var value Visibility
	if err := common.UnmarshalJsonStr(raw, &value); err != nil {
		return Visibility{}, fmt.Errorf("decode group visibility: %w", err)
	}
	return normalizeStructural(value)
}

func validateVisibilityFields(fields map[string]json.RawMessage, allowEnabled bool) error {
	for field := range fields {
		switch field {
		case "version", "default_mode", "rules":
		case "enabled":
			if allowEnabled {
				continue
			}
			fallthrough
		default:
			return fmt.Errorf("decode group visibility: unknown field %q", field)
		}
	}
	if allowEnabled {
		if _, exists := fields["enabled"]; !exists {
			return fmt.Errorf("decode group visibility: missing field %q", "enabled")
		}
	}
	return nil
}

func validateRuleFields(fields map[string]json.RawMessage) error {
	if encodedRules, exists := fields["rules"]; exists {
		var rules map[string]json.RawMessage
		if err := common.Unmarshal(encodedRules, &rules); err != nil {
			return fmt.Errorf("decode group visibility rules: %w", err)
		}
		for group, encodedRule := range rules {
			var ruleFields map[string]json.RawMessage
			if err := common.Unmarshal(encodedRule, &ruleFields); err != nil {
				return fmt.Errorf("decode visibility rule for %q: %w", group, err)
			}
			for field := range ruleFields {
				switch field {
				case "mode", "user_groups":
				default:
					return fmt.Errorf("decode visibility rule for %q: unknown field %q", group, field)
				}
			}
		}
	}
	return nil
}

func normalizeStructural(value Visibility) (Visibility, error) {
	if value.Version != CurrentVersion {
		return Visibility{}, fmt.Errorf("unsupported group visibility version %d", value.Version)
	}
	if value.DefaultMode != DefaultModeAll && value.DefaultMode != DefaultModeHidden {
		return Visibility{}, fmt.Errorf("unsupported default visibility mode %q", value.DefaultMode)
	}

	normalized := Visibility{
		Version:     CurrentVersion,
		Enabled:     value.Enabled,
		DefaultMode: value.DefaultMode,
		Rules:       make(map[string]GroupRule, len(value.Rules)),
	}
	for catalogGroup, rule := range value.Rules {
		if err := validateGroupName(catalogGroup); err != nil {
			return Visibility{}, fmt.Errorf("catalog %w", err)
		}
		if rule.Mode != ModeAll && rule.Mode != ModeSelected {
			return Visibility{}, fmt.Errorf("unsupported visibility mode %q for catalog group %q", rule.Mode, catalogGroup)
		}
		if rule.Mode == ModeAll {
			rule.UserGroups = []string{}
		} else {
			userGroups, err := normalizeGroupNames(rule.UserGroups)
			if err != nil {
				return Visibility{}, fmt.Errorf("catalog group %q: %w", catalogGroup, err)
			}
			rule.UserGroups = userGroups
		}
		normalized.Rules[catalogGroup] = rule
	}
	return normalized, nil
}

func normalizeGroupNames(groups []string) ([]string, error) {
	seen := make(map[string]struct{}, len(groups))
	normalized := make([]string, 0, len(groups))
	for _, group := range groups {
		if err := validateGroupName(group); err != nil {
			return nil, err
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func validateGroupName(group string) error {
	if group == "" || strings.TrimSpace(group) != group {
		return fmt.Errorf("group identifiers must be non-empty exact values")
	}
	if group == "all" || group == "auto" {
		return fmt.Errorf("group %q is an internal sentinel", group)
	}
	return nil
}

func ValidateUpdate(next, current Visibility, catalogUniverse, userGroupUniverse map[string]string) (Visibility, error) {
	normalized, err := normalizeStructural(next)
	if err != nil {
		return Visibility{}, err
	}
	current, err = normalizeStructural(current)
	if err != nil {
		current = DefaultVisibility()
	}

	for catalogGroup, rule := range normalized.Rules {
		currentRule, catalogExistedBefore := current.Rules[catalogGroup]
		if _, exists := catalogUniverse[catalogGroup]; !exists && !catalogExistedBefore {
			return Visibility{}, fmt.Errorf("unknown catalog group %q", catalogGroup)
		}
		staleUserGroups := make(map[string]struct{}, len(currentRule.UserGroups))
		for _, userGroup := range currentRule.UserGroups {
			staleUserGroups[userGroup] = struct{}{}
		}
		for _, userGroup := range rule.UserGroups {
			if _, exists := userGroupUniverse[userGroup]; exists {
				continue
			}
			if _, existedBefore := staleUserGroups[userGroup]; !existedBefore {
				return Visibility{}, fmt.Errorf("unknown user group %q for catalog group %q", userGroup, catalogGroup)
			}
		}
	}
	return normalized, nil
}

func VisibleToUser(value Visibility, catalogGroup, userGroup string) bool {
	if !value.Enabled {
		return true
	}
	rule, exists := value.Rules[catalogGroup]
	if !exists {
		return value.DefaultMode == DefaultModeAll
	}
	if rule.Mode == ModeAll {
		return true
	}
	for _, allowedGroup := range rule.UserGroups {
		if allowedGroup == userGroup {
			return true
		}
	}
	return false
}

func EffectiveRule(value Visibility, catalogGroup string) GroupRule {
	if !value.Enabled {
		return GroupRule{Mode: ModeAll, UserGroups: []string{}}
	}
	if rule, exists := value.Rules[catalogGroup]; exists {
		return rule
	}
	if value.DefaultMode == DefaultModeAll {
		return GroupRule{Mode: ModeAll, UserGroups: []string{}}
	}
	return GroupRule{Mode: ModeSelected, UserGroups: []string{}}
}

func Marshal(value Visibility) (string, error) {
	normalized, err := normalizeStructural(value)
	if err != nil {
		return "", err
	}
	encoded, err := common.Marshal(normalized)
	return string(encoded), err
}

func ETag(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}
