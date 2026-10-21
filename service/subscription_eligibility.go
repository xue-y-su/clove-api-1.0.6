package service

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

func ResolveTokenEffectiveGroups(userGroup, tokenGroup string) []string {
	userGroup = strings.TrimSpace(userGroup)
	tokenGroup = strings.TrimSpace(tokenGroup)

	groups := []string{}
	switch tokenGroup {
	case "auto":
		groups = GetUserAutoGroup(userGroup)
	case "":
		if userGroup != "" {
			groups = []string{userGroup}
		}
	default:
		groups = []string{tokenGroup}
	}

	result := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		result = append(result, group)
	}
	return result
}

func ResolveRequestEffectiveGroups(userGroup, usingGroup string) []string {
	usingGroup = strings.TrimSpace(usingGroup)
	if usingGroup == "" {
		return ResolveTokenEffectiveGroups(userGroup, "")
	}
	return ResolveTokenEffectiveGroups(userGroup, usingGroup)
}

func SubscriptionAppliesToGroups(subscriptionGroup string, effectiveGroups []string) bool {
	subscriptionGroup = strings.TrimSpace(subscriptionGroup)
	if subscriptionGroup == "" {
		return true
	}
	if len(effectiveGroups) == 0 {
		return false
	}

	for _, effectiveGroup := range effectiveGroups {
		effectiveGroup = strings.TrimSpace(effectiveGroup)
		if effectiveGroup == subscriptionGroup {
			return true
		}
	}
	return false
}

func SubscriptionEligibilityForGroups(effectiveGroups []string) model.SubscriptionEligibility {
	groups := append([]string(nil), effectiveGroups...)
	return func(subscriptionGroup string) bool {
		return SubscriptionAppliesToGroups(subscriptionGroup, groups)
	}
}
