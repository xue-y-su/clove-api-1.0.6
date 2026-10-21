package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"gorm.io/gorm"
)

type PricingSourceSnapshot struct {
	Generation         uint64
	Pricing            []Pricing
	Vendors            []PricingVendor
	SupportedEndpoints map[string]common.EndpointInfo
	GroupRatio         map[string]float64
	GroupDescriptions  map[string]string
}

type CatalogPricingSnapshot struct {
	Pricing            []Pricing
	Vendors            []PricingVendor
	GroupRatio         map[string]float64
	AllGroup           map[string]string
	SupportedEndpoints map[string]common.EndpointInfo
	Version            string
	Visibility         model_catalog_setting.Visibility
	VisibilityETag     string
	SourceGeneration   uint64
	InvalidationEpoch  uint64
}

var (
	catalogPricingPointer    atomic.Pointer[CatalogPricingSnapshot]
	catalogAdminStatePointer atomic.Pointer[CatalogAdminState]
	catalogInvalidationEpoch atomic.Uint64
	catalogPricingBuildLock  sync.Mutex
	catalogMalformedWarnings sync.Map
	loadCatalogPricingSource = GetPricingSourceSnapshot
	loadCatalogVisibility    = loadPersistedCatalogVisibility
	loadCatalogUserGroups    = UserGroupUniverse
)

var ErrCatalogVisibilityConflict = errors.New("catalog visibility changed")

type CatalogAdminState struct {
	Config      *model_catalog_setting.Visibility `json:"config,omitempty"`
	ETag        string                            `json:"etag"`
	ConfigError string                            `json:"config_error,omitempty"`
	Groups      []CatalogAdminGroup               `json:"groups"`
	UserGroups  []CatalogAdminGroup               `json:"user_groups"`
	Summary     CatalogSummary                    `json:"summary"`
}

type CatalogAdminGroup struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Exists      bool   `json:"exists"`
}

type CatalogSummary struct {
	GroupCount           int    `json:"group_count"`
	ModelCount           int    `json:"model_count"`
	RestrictedGroupCount int    `json:"restricted_group_count"`
	PricingVersion       string `json:"pricing_version"`
}

type CatalogVisibilityChange struct {
	OldMode      string
	NewMode      string
	OldEnabled   bool
	NewEnabled   bool
	AddedCount   int
	RemovedCount int
	ConfigDigest string
}

func GetCatalogPricingSnapshot() (CatalogPricingSnapshot, error) {
	return GetCatalogPricingSnapshotForUserGroup("")
}

func GetCatalogPricingSnapshotForUserGroup(userGroup string) (CatalogPricingSnapshot, error) {
	base, err := getCatalogPricingBaseSnapshot()
	if err != nil {
		return CatalogPricingSnapshot{}, err
	}
	projected, err := buildCatalogProjectionForUserGroup(
		pricingSourceFromCatalogSnapshot(base),
		base.Visibility,
		userGroup,
	)
	if err != nil {
		return CatalogPricingSnapshot{}, err
	}
	projected.VisibilityETag = base.VisibilityETag
	projected.InvalidationEpoch = base.InvalidationEpoch
	return projected, nil
}

func getCatalogPricingBaseSnapshot() (CatalogPricingSnapshot, error) {
	source, err := loadCatalogPricingSource()
	if err != nil {
		if retained := catalogPricingPointer.Load(); retained != nil {
			return cloneCatalogPricingSnapshot(*retained), nil
		}
		return CatalogPricingSnapshot{}, err
	}

	common.OptionMapRWMutex.RLock()
	rawVisibility := common.OptionMap[model_catalog_setting.OptionKey]
	common.OptionMapRWMutex.RUnlock()
	visibilityETag := model_catalog_setting.ETag(rawVisibility)
	epoch := catalogInvalidationEpoch.Load()
	if cached := catalogPricingPointer.Load(); cached != nil &&
		cached.SourceGeneration == source.Generation &&
		cached.VisibilityETag == visibilityETag &&
		cached.InvalidationEpoch == epoch {
		return cloneCatalogPricingSnapshot(*cached), nil
	}

	catalogPricingBuildLock.Lock()
	defer catalogPricingBuildLock.Unlock()
	if cached := catalogPricingPointer.Load(); cached != nil &&
		cached.SourceGeneration == source.Generation &&
		cached.VisibilityETag == visibilityETag &&
		cached.InvalidationEpoch == epoch {
		return cloneCatalogPricingSnapshot(*cached), nil
	}

	visibility, parseErr := model_catalog_setting.ParseStored(rawVisibility)
	if parseErr != nil {
		visibility = model_catalog_setting.HiddenByDefaultVisibility()
		if _, warned := catalogMalformedWarnings.LoadOrStore(visibilityETag, struct{}{}); !warned {
			common.SysError(fmt.Sprintf("invalid model catalog visibility setting (digest %s); serving an empty catalog", visibilityETag[:12]))
		}
	}
	snapshot, err := buildCatalogProjectionForUserGroup(
		source,
		model_catalog_setting.EnabledVisibility(),
		"",
	)
	if err != nil {
		if retained := catalogPricingPointer.Load(); retained != nil {
			return cloneCatalogPricingSnapshot(*retained), nil
		}
		return CatalogPricingSnapshot{}, err
	}
	snapshot.Visibility = cloneCatalogVisibility(visibility)
	snapshot.VisibilityETag = visibilityETag
	snapshot.InvalidationEpoch = epoch
	published := cloneCatalogPricingSnapshot(snapshot)
	catalogPricingPointer.Store(&published)
	return cloneCatalogPricingSnapshot(published), nil
}

func InvalidateCatalogPricing() {
	catalogInvalidationEpoch.Add(1)
}

func GetCatalogAdminState() (CatalogAdminState, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		state, err := buildCatalogAdminState()
		if err == nil {
			published := cloneCatalogAdminState(state)
			catalogAdminStatePointer.Store(&published)
			return cloneCatalogAdminState(published), nil
		}
		lastErr = err
		if attempt < 3 {
			time.Sleep(50 * time.Millisecond * time.Duration(1<<attempt))
		}
	}
	if retained := catalogAdminStatePointer.Load(); retained != nil {
		return cloneCatalogAdminState(*retained), nil
	}
	return CatalogAdminState{}, lastErr
}

func buildCatalogAdminState() (CatalogAdminState, error) {
	raw, _, err := loadCatalogVisibility()
	if err != nil {
		return CatalogAdminState{}, err
	}
	source, err := loadCatalogPricingSource()
	if err != nil {
		return CatalogAdminState{}, err
	}
	userGroupUniverse, err := loadCatalogUserGroups()
	if err != nil {
		return CatalogAdminState{}, err
	}
	visibility, parseErr := model_catalog_setting.ParseStored(raw)
	effective := visibility
	if parseErr != nil {
		effective = model_catalog_setting.HiddenByDefaultVisibility()
	}
	snapshot, err := buildCatalogProjectionForUserGroup(
		source,
		model_catalog_setting.EnabledVisibility(),
		"",
	)
	if err != nil {
		return CatalogAdminState{}, err
	}
	snapshot.Visibility = cloneCatalogVisibility(effective)
	snapshot.VisibilityETag = model_catalog_setting.ETag(raw)
	return catalogAdminState(raw, effective, parseErr, source.GroupDescriptions, userGroupUniverse, snapshot), nil
}

func UpdateCatalogVisibility(expectedETag string, next model_catalog_setting.Visibility) (CatalogAdminState, CatalogVisibilityChange, error) {
	catalogPricingBuildLock.Lock()
	defer catalogPricingBuildLock.Unlock()

	currentRaw, existed, err := loadPersistedCatalogVisibility()
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	if model_catalog_setting.ETag(currentRaw) != expectedETag {
		return CatalogAdminState{}, CatalogVisibilityChange{}, ErrCatalogVisibilityConflict
	}
	current, currentErr := model_catalog_setting.ParseStored(currentRaw)
	if currentErr != nil {
		current = model_catalog_setting.DefaultVisibility()
	}
	catalogUniverse, err := CatalogGroupUniverse()
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	userGroupUniverse, err := UserGroupUniverse()
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	normalized, err := model_catalog_setting.ValidateUpdate(next, current, catalogUniverse, userGroupUniverse)
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	nextRaw, err := model_catalog_setting.Marshal(normalized)
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	source, err := loadCatalogPricingSource()
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	candidate, err := buildCatalogProjectionForUserGroup(
		source,
		model_catalog_setting.EnabledVisibility(),
		"",
	)
	if err != nil {
		return CatalogAdminState{}, CatalogVisibilityChange{}, err
	}
	candidate.Visibility = cloneCatalogVisibility(normalized)
	candidate.VisibilityETag = model_catalog_setting.ETag(nextRaw)

	if !existed || currentRaw != nextRaw {
		err = DB.Transaction(func(tx *gorm.DB) error {
			if existed {
				result := tx.Model(&Option{}).
					Where(commonKeyCol+" = ? AND value = ?", model_catalog_setting.OptionKey, currentRaw).
					Update("value", nextRaw)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrCatalogVisibilityConflict
				}
				return nil
			}
			if err := tx.Create(&Option{Key: model_catalog_setting.OptionKey, Value: nextRaw}).Error; err != nil {
				return ErrCatalogVisibilityConflict
			}
			return nil
		})
		if err != nil {
			return CatalogAdminState{}, CatalogVisibilityChange{}, err
		}

		common.OptionMapRWMutex.Lock()
		if common.OptionMap == nil {
			common.OptionMap = make(map[string]string)
		}
		common.OptionMap[model_catalog_setting.OptionKey] = nextRaw
		common.OptionMapRWMutex.Unlock()
		epoch := catalogInvalidationEpoch.Add(1)
		candidate.InvalidationEpoch = epoch
		published := cloneCatalogPricingSnapshot(candidate)
		catalogPricingPointer.Store(&published)
	} else {
		candidate.InvalidationEpoch = catalogInvalidationEpoch.Load()
	}

	change := catalogVisibilityChange(current, normalized, candidate.Version, nextRaw)
	state := catalogAdminState(nextRaw, normalized, nil, source.GroupDescriptions, userGroupUniverse, candidate)
	publishedState := cloneCatalogAdminState(state)
	catalogAdminStatePointer.Store(&publishedState)
	return state, change, nil
}

func loadPersistedCatalogVisibility() (string, bool, error) {
	var option Option
	err := DB.Where(commonKeyCol+" = ?", model_catalog_setting.OptionKey).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return option.Value, true, nil
}

func catalogAdminState(
	raw string,
	visibility model_catalog_setting.Visibility,
	configErr error,
	catalogUniverse map[string]string,
	userGroupUniverse map[string]string,
	snapshot CatalogPricingSnapshot,
) CatalogAdminState {
	state := CatalogAdminState{
		ETag: model_catalog_setting.ETag(raw),
		Summary: CatalogSummary{
			GroupCount:     len(snapshot.AllGroup),
			ModelCount:     len(snapshot.Pricing),
			PricingVersion: snapshot.Version,
		},
	}
	if configErr == nil {
		configCopy := cloneCatalogVisibility(visibility)
		state.Config = &configCopy
	} else {
		state.ConfigError = configErr.Error()
	}
	groupNames := make(map[string]struct{}, len(catalogUniverse)+len(visibility.Rules))
	userGroupNames := make(map[string]struct{}, len(userGroupUniverse))
	for group := range catalogUniverse {
		groupNames[group] = struct{}{}
	}
	for group := range userGroupUniverse {
		userGroupNames[group] = struct{}{}
	}
	for group, rule := range visibility.Rules {
		groupNames[group] = struct{}{}
		for _, userGroup := range rule.UserGroups {
			userGroupNames[userGroup] = struct{}{}
		}
	}
	for group := range groupNames {
		description, exists := catalogUniverse[group]
		if description == "" {
			description = group
		}
		state.Groups = append(state.Groups, CatalogAdminGroup{
			Name:        group,
			Description: description,
			Exists:      exists,
		})
		if model_catalog_setting.EffectiveRule(visibility, group).Mode == model_catalog_setting.ModeSelected {
			state.Summary.RestrictedGroupCount++
		}
	}
	sort.Slice(state.Groups, func(i, j int) bool {
		return state.Groups[i].Name < state.Groups[j].Name
	})
	for group := range userGroupNames {
		description, exists := userGroupUniverse[group]
		if description == "" {
			description = group
		}
		state.UserGroups = append(state.UserGroups, CatalogAdminGroup{
			Name:        group,
			Description: description,
			Exists:      exists,
		})
	}
	sort.Slice(state.UserGroups, func(i, j int) bool {
		return state.UserGroups[i].Name < state.UserGroups[j].Name
	})
	return state
}

func catalogVisibilityChange(
	current model_catalog_setting.Visibility,
	next model_catalog_setting.Visibility,
	version string,
	raw string,
) CatalogVisibilityChange {
	added := 0
	for group, nextRule := range next.Rules {
		currentRule, existed := current.Rules[group]
		if !existed || !catalogVisibilityRuleEqual(currentRule, nextRule) {
			added++
		}
	}
	removed := 0
	for group, currentRule := range current.Rules {
		nextRule, exists := next.Rules[group]
		if !exists || !catalogVisibilityRuleEqual(currentRule, nextRule) {
			removed++
		}
	}
	return CatalogVisibilityChange{
		OldMode:      current.DefaultMode,
		NewMode:      next.DefaultMode,
		OldEnabled:   current.Enabled,
		NewEnabled:   next.Enabled,
		AddedCount:   added,
		RemovedCount: removed,
		ConfigDigest: model_catalog_setting.ETag(raw + version),
	}
}

func catalogVisibilityRuleEqual(left, right model_catalog_setting.GroupRule) bool {
	if left.Mode != right.Mode || len(left.UserGroups) != len(right.UserGroups) {
		return false
	}
	for index := range left.UserGroups {
		if left.UserGroups[index] != right.UserGroups[index] {
			return false
		}
	}
	return true
}

func isRealCatalogGroup(group string) bool {
	return group != "" && group != "all" && group != "auto"
}

func visibleCatalogGroups(
	source PricingSourceSnapshot,
	visibility model_catalog_setting.Visibility,
	userGroup string,
) map[string]struct{} {
	universe := make(map[string]struct{})
	for group := range source.GroupDescriptions {
		if isRealCatalogGroup(group) {
			universe[group] = struct{}{}
		}
	}
	for group := range source.GroupRatio {
		if isRealCatalogGroup(group) {
			universe[group] = struct{}{}
		}
	}
	for _, item := range source.Pricing {
		for _, group := range item.EnableGroup {
			if isRealCatalogGroup(group) {
				universe[group] = struct{}{}
			}
		}
	}
	visible := make(map[string]struct{}, len(universe))
	for group := range universe {
		if model_catalog_setting.VisibleToUser(visibility, group, userGroup) {
			visible[group] = struct{}{}
		}
	}
	return visible
}

func projectCatalogModelGroups(groups []string, visible map[string]struct{}) []string {
	projectedSet := make(map[string]struct{})
	expandAll := false
	for _, group := range groups {
		if group == "all" {
			expandAll = true
			break
		}
	}
	if expandAll {
		for group := range visible {
			projectedSet[group] = struct{}{}
		}
	} else {
		for _, group := range groups {
			if _, exists := visible[group]; exists {
				projectedSet[group] = struct{}{}
			}
		}
	}
	projected := make([]string, 0, len(projectedSet))
	for group := range projectedSet {
		projected = append(projected, group)
	}
	sort.Strings(projected)
	return projected
}

func buildCatalogProjection(source PricingSourceSnapshot, visibility model_catalog_setting.Visibility) (CatalogPricingSnapshot, error) {
	return buildCatalogProjectionForUserGroup(source, visibility, "")
}

func buildCatalogProjectionForUserGroup(
	source PricingSourceSnapshot,
	visibility model_catalog_setting.Visibility,
	userGroup string,
) (CatalogPricingSnapshot, error) {
	visible := visibleCatalogGroups(source, visibility, userGroup)
	pricing := make([]Pricing, 0, len(source.Pricing))
	usedVendors := make(map[int]struct{})
	usedEndpoints := make(map[string]struct{})
	legacyPricingVersion := ""
	for _, sourceItem := range source.Pricing {
		if sourceItem.PricingVersion != "" &&
			(legacyPricingVersion == "" || sourceItem.PricingVersion < legacyPricingVersion) {
			legacyPricingVersion = sourceItem.PricingVersion
		}
		groups := projectCatalogModelGroups(sourceItem.EnableGroup, visible)
		if len(groups) == 0 {
			continue
		}
		item := clonePricing(sourceItem)
		item.EnableGroup = groups
		item.PricingVersion = ""
		pricing = append(pricing, item)
		if item.VendorID != 0 {
			usedVendors[item.VendorID] = struct{}{}
		}
		for _, endpoint := range item.SupportedEndpointTypes {
			usedEndpoints[string(endpoint)] = struct{}{}
		}
	}
	sort.Slice(pricing, func(i, j int) bool {
		return pricing[i].ModelName < pricing[j].ModelName
	})
	if len(pricing) > 0 {
		pricing[0].PricingVersion = legacyPricingVersion
	}

	allGroup := make(map[string]string, len(visible))
	groupRatio := make(map[string]float64, len(visible))
	for group := range visible {
		allGroup[group] = source.GroupDescriptions[group]
		if allGroup[group] == "" {
			allGroup[group] = group
		}
		if ratio, exists := source.GroupRatio[group]; exists {
			groupRatio[group] = ratio
		}
	}

	vendors := make([]PricingVendor, 0, len(usedVendors))
	for _, vendor := range source.Vendors {
		if _, used := usedVendors[vendor.ID]; used {
			vendors = append(vendors, vendor)
		}
	}
	sort.Slice(vendors, func(i, j int) bool {
		return vendors[i].ID < vendors[j].ID
	})

	supportedEndpoints := make(map[string]common.EndpointInfo, len(usedEndpoints))
	for endpoint := range usedEndpoints {
		if info, exists := source.SupportedEndpoints[endpoint]; exists {
			supportedEndpoints[endpoint] = info
		}
	}

	normalizedVisibility, err := model_catalog_setting.Marshal(visibility)
	if err != nil {
		return CatalogPricingSnapshot{}, err
	}
	snapshot := CatalogPricingSnapshot{
		Pricing:            pricing,
		Vendors:            vendors,
		GroupRatio:         groupRatio,
		AllGroup:           allGroup,
		SupportedEndpoints: supportedEndpoints,
		Visibility:         visibility,
		VisibilityETag:     model_catalog_setting.ETag(normalizedVisibility),
		SourceGeneration:   source.Generation,
	}
	encoded, err := common.Marshal(struct {
		Pricing            []Pricing                      `json:"pricing"`
		Vendors            []PricingVendor                `json:"vendors"`
		GroupRatio         map[string]float64             `json:"group_ratio"`
		AllGroup           map[string]string              `json:"all_group"`
		SupportedEndpoints map[string]common.EndpointInfo `json:"supported_endpoints"`
	}{
		Pricing:            snapshot.Pricing,
		Vendors:            snapshot.Vendors,
		GroupRatio:         snapshot.GroupRatio,
		AllGroup:           snapshot.AllGroup,
		SupportedEndpoints: snapshot.SupportedEndpoints,
	})
	if err != nil {
		return CatalogPricingSnapshot{}, err
	}
	digest := sha256.Sum256(encoded)
	snapshot.Version = hex.EncodeToString(digest[:])
	return snapshot, nil
}

func pricingSourceFromCatalogSnapshot(snapshot CatalogPricingSnapshot) PricingSourceSnapshot {
	return PricingSourceSnapshot{
		Generation:         snapshot.SourceGeneration,
		Pricing:            snapshot.Pricing,
		Vendors:            snapshot.Vendors,
		SupportedEndpoints: snapshot.SupportedEndpoints,
		GroupRatio:         snapshot.GroupRatio,
		GroupDescriptions:  snapshot.AllGroup,
	}
}

func clonePricing(source Pricing) Pricing {
	clone := source
	clone.EnableGroup = append([]string(nil), source.EnableGroup...)
	clone.SupportedEndpointTypes = append([]constant.EndpointType(nil), source.SupportedEndpointTypes...)
	clone.CacheRatio = clonePricingRatio(source.CacheRatio)
	clone.CreateCacheRatio = clonePricingRatio(source.CreateCacheRatio)
	clone.ImageRatio = clonePricingRatio(source.ImageRatio)
	clone.AudioRatio = clonePricingRatio(source.AudioRatio)
	clone.AudioCompletionRatio = clonePricingRatio(source.AudioCompletionRatio)
	return clone
}

func clonePricingRatio(source *float64) *float64 {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}

func cloneCatalogPricingSnapshot(source CatalogPricingSnapshot) CatalogPricingSnapshot {
	clone := source
	clone.Pricing = make([]Pricing, len(source.Pricing))
	for index, item := range source.Pricing {
		clone.Pricing[index] = clonePricing(item)
	}
	clone.Vendors = append([]PricingVendor(nil), source.Vendors...)
	clone.GroupRatio = make(map[string]float64, len(source.GroupRatio))
	for group, ratio := range source.GroupRatio {
		clone.GroupRatio[group] = ratio
	}
	clone.AllGroup = make(map[string]string, len(source.AllGroup))
	for group, description := range source.AllGroup {
		clone.AllGroup[group] = description
	}
	clone.SupportedEndpoints = make(map[string]common.EndpointInfo, len(source.SupportedEndpoints))
	for endpoint, info := range source.SupportedEndpoints {
		clone.SupportedEndpoints[endpoint] = info
	}
	clone.Visibility = cloneCatalogVisibility(source.Visibility)
	return clone
}

func cloneCatalogAdminState(source CatalogAdminState) CatalogAdminState {
	clone := source
	if source.Config != nil {
		config := cloneCatalogVisibility(*source.Config)
		clone.Config = &config
	}
	clone.Groups = append([]CatalogAdminGroup(nil), source.Groups...)
	clone.UserGroups = append([]CatalogAdminGroup(nil), source.UserGroups...)
	return clone
}

func cloneCatalogVisibility(source model_catalog_setting.Visibility) model_catalog_setting.Visibility {
	clone := source
	clone.Rules = make(map[string]model_catalog_setting.GroupRule, len(source.Rules))
	for group, rule := range source.Rules {
		rule.UserGroups = append([]string(nil), rule.UserGroups...)
		clone.Rules[group] = rule
	}
	return clone
}

func clonePricingSourceSnapshot(source PricingSourceSnapshot) PricingSourceSnapshot {
	clone := source
	clone.Pricing = make([]Pricing, len(source.Pricing))
	for index, item := range source.Pricing {
		clone.Pricing[index] = clonePricing(item)
	}
	clone.Vendors = append([]PricingVendor(nil), source.Vendors...)
	clone.SupportedEndpoints = make(map[string]common.EndpointInfo, len(source.SupportedEndpoints))
	for endpoint, info := range source.SupportedEndpoints {
		clone.SupportedEndpoints[endpoint] = info
	}
	clone.GroupRatio = make(map[string]float64, len(source.GroupRatio))
	for group, ratio := range source.GroupRatio {
		clone.GroupRatio[group] = ratio
	}
	clone.GroupDescriptions = make(map[string]string, len(source.GroupDescriptions))
	for group, description := range source.GroupDescriptions {
		clone.GroupDescriptions[group] = description
	}
	return clone
}
