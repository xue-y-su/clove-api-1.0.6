package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetPricing(c *gin.Context) {
	userId, exists := c.Get("id")
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
		}
	}
	userUsableGroups := service.GetUserUsableGroups(group)
	snapshot, err := model.GetCatalogPricingSnapshotForUserGroup(group)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "model catalog is temporarily unavailable",
		})
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Vary", "Cookie, Authorization")

	pricing := snapshot.Pricing
	vendors := snapshot.Vendors
	catalogGroups := snapshot.AllGroup
	supportedEndpoints := snapshot.SupportedEndpoints
	if !snapshot.Visibility.Enabled {
		pricing = filterPricingByUsableGroups(model.GetPricing(), userUsableGroups)
		vendors = model.GetVendors()
		catalogGroups = intersectCatalogGroups(userUsableGroups, snapshot.AllGroup)
		supportedEndpoints = model.GetSupportedEndpointMap()
	}

	groupRatio := make(map[string]float64, len(catalogGroups))
	for target := range catalogGroups {
		groupRatio[target] = snapshot.GroupRatio[target]
		if ratio, ok := ratio_setting.GetGroupGroupRatio(group, target); ok {
			groupRatio[target] = ratio
		}
	}
	usableGroup := intersectCatalogGroups(userUsableGroups, catalogGroups)
	autoGroups := make([]string, 0)
	for _, autoGroup := range service.GetUserAutoGroup(group) {
		if _, visible := catalogGroups[autoGroup]; visible {
			autoGroups = append(autoGroups, autoGroup)
		}
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            vendors,
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"all_group":          catalogGroups,
		"supported_endpoint": supportedEndpoints,
		"auto_groups":        autoGroups,
		"pricing_version":    snapshot.Version,
	})
}

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroups map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroups) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		for _, group := range item.EnableGroup {
			if group == "all" {
				filtered = append(filtered, item)
				break
			}
			if _, usable := usableGroups[group]; usable {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func intersectCatalogGroups(source, visible map[string]string) map[string]string {
	result := make(map[string]string)
	for group, description := range source {
		if _, exists := visible[group]; exists {
			result[group] = description
		}
	}
	return result
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
