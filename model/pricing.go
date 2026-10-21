package model

import (
	"fmt"
	"strings"

	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Pricing struct {
	ModelName              string                  `json:"model_name"`
	Description            string                  `json:"description,omitempty"`
	Icon                   string                  `json:"icon,omitempty"`
	Tags                   string                  `json:"tags,omitempty"`
	VendorID               int                     `json:"vendor_id,omitempty"`
	QuotaType              int                     `json:"quota_type"`
	ModelRatio             float64                 `json:"model_ratio"`
	ModelPrice             float64                 `json:"model_price"`
	OwnerBy                string                  `json:"owner_by"`
	CompletionRatio        float64                 `json:"completion_ratio"`
	CacheRatio             *float64                `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                `json:"create_cache_ratio,omitempty"`
	ImageRatio             *float64                `json:"image_ratio,omitempty"`
	AudioRatio             *float64                `json:"audio_ratio,omitempty"`
	AudioCompletionRatio   *float64                `json:"audio_completion_ratio,omitempty"`
	EnableGroup            []string                `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType `json:"supported_endpoint_types"`
	BillingMode            string                  `json:"billing_mode,omitempty"`
	BillingExpr            string                  `json:"billing_expr,omitempty"`
	PricingVersion         string                  `json:"pricing_version,omitempty"`
}

type PricingVendor struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

var (
	pricingMap           []Pricing
	vendorsList          []PricingVendor
	supportedEndpointMap map[string]common.EndpointInfo
	lastGetPricingTime   time.Time
	updatePricingLock    sync.Mutex
	pricingGeneration    atomic.Uint64

	// 缓存映射：模型名 -> 启用分组 / 计费类型
	modelEnableGroups     = make(map[string][]string)
	modelQuotaTypeMap     = make(map[string]int)
	modelEnableGroupsLock = sync.RWMutex{}
)

var loadPricingAbilities = GetAllEnableAbilityWithChannels

var explicitCatalogPricingOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"billing_setting.billing_mode",
	"billing_setting.billing_expr",
}

var (
	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	modelSupportEndpointsLock = sync.RWMutex{}
)

func GetPricing() []Pricing {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()
	if err := ensurePricingLocked(); err != nil {
		common.SysLog(fmt.Sprintf("refresh pricing error: %v", err))
	}
	return pricingMap
}

func ensurePricingLocked() error {
	if time.Since(lastGetPricingTime) <= time.Minute && len(pricingMap) > 0 {
		return nil
	}
	modelSupportEndpointsLock.Lock()
	defer modelSupportEndpointsLock.Unlock()
	return updatePricing()
}

func InvalidatePricingCache() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	lastGetPricingTime = time.Time{}
	InvalidateCatalogPricing()
}

// GetVendors 返回当前定价接口使用到的供应商信息
func GetVendors() []PricingVendor {
	if time.Since(lastGetPricingTime) > time.Minute*1 || len(pricingMap) == 0 {
		// 保证先刷新一次
		GetPricing()
	}
	return vendorsList
}

func GetModelSupportEndpointTypes(model string) []constant.EndpointType {
	if model == "" {
		return make([]constant.EndpointType, 0)
	}
	modelSupportEndpointsLock.RLock()
	defer modelSupportEndpointsLock.RUnlock()
	if endpoints, ok := modelSupportEndpointTypes[model]; ok {
		return endpoints
	}
	return make([]constant.EndpointType, 0)
}

func getPricingEndpointTypesForAbility(ability AbilityWithChannel, advancedCustomConfigs map[int]*dto.AdvancedCustomConfig) []constant.EndpointType {
	if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
		return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
	}
	if config := advancedCustomConfigs[ability.ChannelId]; config != nil {
		return config.SupportedEndpointTypesForModel(ability.Model)
	}
	return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
}

// loadPricingAdvancedCustomConfigs runs inside updatePricing while
// updatePricingLock is held, and nests channelSyncLock.RLock. This defines the
// global lock order updatePricingLock -> channelSyncLock: any code path holding
// channelSyncLock must release it before touching the pricing cache (see
// InitChannelCache / CacheUpdateChannel), otherwise it deadlocks.
// The returned configs are pointers shared with the channel cache; they are
// replaced wholesale on update and never mutated in place, so reading them after
// RUnlock is safe.
func loadPricingAdvancedCustomConfigs(enableAbilities []AbilityWithChannel) map[int]*dto.AdvancedCustomConfig {
	channelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, ability := range enableAbilities {
		if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	if len(channelIDs) == 0 {
		return nil
	}

	configs := make(map[int]*dto.AdvancedCustomConfig, len(channelIDs))
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		for _, channelID := range channelIDs {
			if config := channel2advancedCustomConfig[channelID]; config != nil {
				configs[channelID] = config
			}
		}
		return configs
	}

	for _, channelID := range channelIDs {
		channel, err := CacheGetChannel(channelID)
		if err != nil {
			common.SysLog(fmt.Sprintf("load advanced custom channel settings error: channel_id=%d, error=%v", channelID, err))
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			configs[channelID] = config
		}
	}
	return configs
}

func appendPricingEndpoint(endpoints []string, endpoint string) []string {
	if endpoint == "" || common.StringsContains(endpoints, endpoint) {
		return endpoints
	}
	return append(endpoints, endpoint)
}

func normalizeExplicitCatalogModel(modelName string) string {
	return ratio_setting.FormatMatchingModelName(strings.TrimSpace(modelName))
}

func explicitCatalogPricingOptionsQuery(db *gorm.DB) *gorm.DB {
	keys := make([]interface{}, len(explicitCatalogPricingOptionKeys))
	for index, key := range explicitCatalogPricingOptionKeys {
		keys[index] = key
	}
	return db.Where(clause.IN{Column: clause.Column{Name: "key"}, Values: keys})
}

func loadExplicitCatalogPricingOptions(db *gorm.DB) ([]Option, error) {
	var options []Option
	err := explicitCatalogPricingOptionsQuery(db).Find(&options).Error
	return options, err
}

func loadExplicitCatalogPricingModels() (map[string]struct{}, error) {
	explicitRatiosOrPrices := make(map[string]struct{})
	eligible := make(map[string]struct{})
	if DB == nil || !DB.Migrator().HasTable(&Option{}) {
		return eligible, nil
	}

	options, err := loadExplicitCatalogPricingOptions(DB)
	if err != nil {
		return nil, err
	}
	billingModes := make(map[string]string)
	billingExpressions := make(map[string]string)
	for _, option := range options {
		if strings.TrimSpace(option.Value) == "" {
			continue
		}
		switch option.Key {
		case "ModelPrice", "ModelRatio":
			values := make(map[string]float64)
			if err := common.Unmarshal([]byte(option.Value), &values); err != nil {
				return nil, fmt.Errorf("parse persisted %s: %w", option.Key, err)
			}
			for modelName := range values {
				if normalized := normalizeExplicitCatalogModel(modelName); normalized != "" {
					explicitRatiosOrPrices[normalized] = struct{}{}
				}
			}
		case "billing_setting.billing_mode":
			values := make(map[string]string)
			if err := common.Unmarshal([]byte(option.Value), &values); err != nil {
				return nil, fmt.Errorf("parse persisted billing modes: %w", err)
			}
			for modelName, mode := range values {
				if normalized := normalizeExplicitCatalogModel(modelName); normalized != "" {
					billingModes[normalized] = strings.TrimSpace(mode)
				}
			}
		case "billing_setting.billing_expr":
			values := make(map[string]string)
			if err := common.Unmarshal([]byte(option.Value), &values); err != nil {
				return nil, fmt.Errorf("parse persisted billing expressions: %w", err)
			}
			for modelName, expression := range values {
				if normalized := normalizeExplicitCatalogModel(modelName); normalized != "" {
					billingExpressions[normalized] = strings.TrimSpace(expression)
				}
			}
		}
	}

	for modelName := range explicitRatiosOrPrices {
		if billingModes[modelName] == billing_setting.BillingModeTieredExpr && billingExpressions[modelName] == "" {
			continue
		}
		eligible[modelName] = struct{}{}
	}
	for modelName, mode := range billingModes {
		if mode == billing_setting.BillingModeTieredExpr && billingExpressions[modelName] != "" {
			eligible[modelName] = struct{}{}
		}
	}
	return eligible, nil
}

func catalogEligibleAbility(ability AbilityWithChannel, explicitlyPriced map[string]struct{}) bool {
	if ability.ChannelType != constant.ChannelTypeSub2API {
		return true
	}
	if !isRealCatalogGroup(ability.Group) || !IsChannelModelPublicationApproved(ability.ChannelId, ability.Model) {
		return false
	}
	_, ok := explicitlyPriced[normalizeExplicitCatalogModel(ability.Model)]
	return ok
}

func updatePricing() error {
	//modelRatios := common.GetModelRatios()
	enableAbilities, err := loadPricingAbilities()
	if err != nil {
		common.SysLog(fmt.Sprintf("GetAllEnableAbilityWithChannels error: %v", err))
		return err
	}
	explicitlyPriced, err := loadExplicitCatalogPricingModels()
	if err != nil {
		common.SysLog(fmt.Sprintf("load explicit catalog pricing error: %v", err))
		return err
	}
	eligibleAbilities := make([]AbilityWithChannel, 0, len(enableAbilities))
	for _, ability := range enableAbilities {
		if catalogEligibleAbility(ability, explicitlyPriced) {
			eligibleAbilities = append(eligibleAbilities, ability)
		}
	}
	enableAbilities = eligibleAbilities
	// 预加载模型元数据与供应商一次，避免循环查询
	var allMeta []Model
	_ = DB.Find(&allMeta).Error
	metaMap := make(map[string]*Model)
	prefixList := make([]*Model, 0)
	suffixList := make([]*Model, 0)
	containsList := make([]*Model, 0)
	for i := range allMeta {
		m := &allMeta[i]
		if m.NameRule == NameRuleExact {
			metaMap[m.ModelName] = m
		} else {
			switch m.NameRule {
			case NameRulePrefix:
				prefixList = append(prefixList, m)
			case NameRuleSuffix:
				suffixList = append(suffixList, m)
			case NameRuleContains:
				containsList = append(containsList, m)
			}
		}
	}

	// 将非精确规则模型匹配到 metaMap
	for _, m := range prefixList {
		for _, pricingModel := range enableAbilities {
			if strings.HasPrefix(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}
	for _, m := range suffixList {
		for _, pricingModel := range enableAbilities {
			if strings.HasSuffix(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}
	for _, m := range containsList {
		for _, pricingModel := range enableAbilities {
			if strings.Contains(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}

	// 预加载供应商
	var vendors []Vendor
	_ = DB.Find(&vendors).Error
	vendorMap := make(map[int]*Vendor)
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	// 初始化默认供应商映射
	initDefaultVendorMapping(metaMap, vendorMap, enableAbilities)

	// 构建对前端友好的供应商列表
	vendorsList = make([]PricingVendor, 0, len(vendorMap))
	for _, v := range vendorMap {
		vendorsList = append(vendorsList, PricingVendor{
			ID:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			Icon:        v.Icon,
		})
	}

	modelGroupsMap := make(map[string]*types.Set[string])

	for _, ability := range enableAbilities {
		groups, ok := modelGroupsMap[ability.Model]
		if !ok {
			groups = types.NewSet[string]()
			modelGroupsMap[ability.Model] = groups
		}
		groups.Add(ability.Group)
	}

	//这里使用切片而不是Set，因为一个模型可能支持多个端点类型，并且第一个端点是优先使用端点
	modelSupportEndpointsStr := make(map[string][]string)
	advancedCustomConfigs := loadPricingAdvancedCustomConfigs(enableAbilities)

	// 先根据已有能力填充原生端点
	for _, ability := range enableAbilities {
		endpoints := modelSupportEndpointsStr[ability.Model]
		channelTypes := getPricingEndpointTypesForAbility(ability, advancedCustomConfigs)
		for _, channelType := range channelTypes {
			if !common.StringsContains(endpoints, string(channelType)) {
				endpoints = append(endpoints, string(channelType))
			}
		}
		modelSupportEndpointsStr[ability.Model] = endpoints
	}

	// 再补充模型自定义端点：若配置有效则追加到已有推断，不再裁剪渠道真实能力
	for modelName, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]interface{}
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			endpoints := modelSupportEndpointsStr[modelName]
			for k, v := range raw {
				switch v.(type) {
				case string, map[string]interface{}:
					endpoints = appendPricingEndpoint(endpoints, k)
				}
			}
			if len(endpoints) > 0 {
				modelSupportEndpointsStr[modelName] = endpoints
			}
		}
	}

	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	for model, endpoints := range modelSupportEndpointsStr {
		supportedEndpoints := make([]constant.EndpointType, 0)
		for _, endpointStr := range endpoints {
			endpointType := constant.EndpointType(endpointStr)
			supportedEndpoints = append(supportedEndpoints, endpointType)
		}
		modelSupportEndpointTypes[model] = supportedEndpoints
	}

	// 构建全局 supportedEndpointMap（默认 + 自定义覆盖）
	supportedEndpointMap = make(map[string]common.EndpointInfo)
	// 1. 默认端点
	for _, endpoints := range modelSupportEndpointTypes {
		for _, et := range endpoints {
			if info, ok := common.GetDefaultEndpointInfo(et); ok {
				if _, exists := supportedEndpointMap[string(et)]; !exists {
					supportedEndpointMap[string(et)] = info
				}
			}
		}
	}
	// 2. 自定义端点（models 表）覆盖默认
	for _, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]interface{}
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			for k, v := range raw {
				switch val := v.(type) {
				case string:
					supportedEndpointMap[k] = common.EndpointInfo{Path: val, Method: "POST"}
				case map[string]interface{}:
					ep := common.EndpointInfo{Method: "POST"}
					if p, ok := val["path"].(string); ok {
						ep.Path = p
					}
					if m, ok := val["method"].(string); ok {
						ep.Method = strings.ToUpper(m)
					}
					supportedEndpointMap[k] = ep
				default:
					// ignore unsupported types
				}
			}
		}
	}

	pricingMap = make([]Pricing, 0)
	for model, groups := range modelGroupsMap {
		pricing := Pricing{
			ModelName:              model,
			EnableGroup:            groups.Items(),
			SupportedEndpointTypes: modelSupportEndpointTypes[model],
		}

		// 补充模型元数据（描述、标签、供应商、状态）
		if meta, ok := metaMap[model]; ok {
			// 若模型被禁用(status!=1)，则直接跳过，不返回给前端
			if meta.Status != 1 {
				continue
			}
			pricing.Description = meta.Description
			pricing.Icon = meta.Icon
			pricing.Tags = meta.Tags
			pricing.VendorID = meta.VendorID
		}
		modelPrice, findPrice := ratio_setting.GetModelPrice(model, false)
		if findPrice {
			pricing.ModelPrice = modelPrice
			pricing.QuotaType = 1
		} else {
			modelRatio, _, _ := ratio_setting.GetModelRatio(model)
			pricing.ModelRatio = modelRatio
			pricing.CompletionRatio = ratio_setting.GetCompletionRatio(model)
			pricing.QuotaType = 0
		}
		if cacheRatio, ok := ratio_setting.GetCacheRatio(model); ok {
			pricing.CacheRatio = &cacheRatio
		}
		if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(model); ok {
			pricing.CreateCacheRatio = &createCacheRatio
		}
		if imageRatio, ok := ratio_setting.GetImageRatio(model); ok {
			pricing.ImageRatio = &imageRatio
		}
		if ratio_setting.ContainsAudioRatio(model) {
			audioRatio := ratio_setting.GetAudioRatio(model)
			pricing.AudioRatio = &audioRatio
		}
		if ratio_setting.ContainsAudioCompletionRatio(model) {
			audioCompletionRatio := ratio_setting.GetAudioCompletionRatio(model)
			pricing.AudioCompletionRatio = &audioCompletionRatio
		}
		if billingMode := billing_setting.GetBillingMode(model); billingMode == "tiered_expr" {
			if expr, ok := billing_setting.GetBillingExpr(model); ok && strings.TrimSpace(expr) != "" {
				pricing.BillingMode = billingMode
				pricing.BillingExpr = expr
			}
		}
		pricingMap = append(pricingMap, pricing)
	}

	// 防止大更新后数据不通用
	if len(pricingMap) > 0 {
		pricingMap[0].PricingVersion = "5a90f2b86c08bd983a9a2e6d66c255f4eaef9c4bc934386d2b6ae84ef0ff1f1f"
	}

	// 刷新缓存映射，供高并发快速查询
	modelEnableGroupsLock.Lock()
	modelEnableGroups = make(map[string][]string)
	modelQuotaTypeMap = make(map[string]int)
	for _, p := range pricingMap {
		modelEnableGroups[p.ModelName] = p.EnableGroup
		modelQuotaTypeMap[p.ModelName] = p.QuotaType
	}
	modelEnableGroupsLock.Unlock()

	lastGetPricingTime = time.Now()
	pricingGeneration.Add(1)
	return nil
}

// GetSupportedEndpointMap 返回全局端点到路径的映射
func GetSupportedEndpointMap() map[string]common.EndpointInfo {
	return supportedEndpointMap
}

func GetPricingSourceSnapshot() (PricingSourceSnapshot, error) {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()
	if err := ensurePricingLocked(); err != nil {
		return PricingSourceSnapshot{}, err
	}

	groupDescriptions, err := catalogGroupUniverseFromAbilities()
	if err != nil {
		return PricingSourceSnapshot{}, err
	}
	groupRatio := ratio_setting.GetGroupRatioCopy()
	for group := range groupDescriptions {
		if _, exists := groupRatio[group]; !exists {
			groupRatio[group] = ratio_setting.GetGroupRatio(group)
		}
	}
	return clonePricingSourceSnapshot(PricingSourceSnapshot{
		Generation:         pricingGeneration.Load(),
		Pricing:            pricingMap,
		Vendors:            vendorsList,
		SupportedEndpoints: supportedEndpointMap,
		GroupRatio:         groupRatio,
		GroupDescriptions:  groupDescriptions,
	}), nil
}

func CatalogGroupUniverse() (map[string]string, error) {
	return catalogGroupUniverseFromAbilities()
}

func UserGroupUniverse() (map[string]string, error) {
	groups := setting.GetUserUsableGroupsCopy()
	for group := range ratio_setting.GetGroupRatioCopy() {
		if isRealCatalogGroup(group) {
			groups[group] = setting.GetUsableGroupDescription(group)
		}
	}

	var storedGroups []struct {
		Group string `gorm:"column:group"`
	}
	if err := DB.Model(&User{}).Select(commonGroupCol).Distinct().Scan(&storedGroups).Error; err != nil {
		return nil, err
	}
	for _, storedGroup := range storedGroups {
		group := storedGroup.Group
		if isRealCatalogGroup(group) {
			groups[group] = setting.GetUsableGroupDescription(group)
		}
	}
	return groups, nil
}

func catalogGroupUniverseFromAbilities() (map[string]string, error) {
	groups := make(map[string]string)
	for group := range ratio_setting.GetGroupRatioCopy() {
		if isRealCatalogGroup(group) {
			groups[group] = setting.GetUsableGroupDescription(group)
		}
	}
	abilities, err := loadPricingAbilities()
	if err != nil {
		return nil, err
	}
	for _, ability := range abilities {
		if isRealCatalogGroup(ability.Group) {
			groups[ability.Group] = setting.GetUsableGroupDescription(ability.Group)
		}
	}
	return groups, nil
}
