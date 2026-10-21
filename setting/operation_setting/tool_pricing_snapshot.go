package operation_setting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const (
	ToolMultiplierGroupOnly = "group_only"
	ToolMultiplierFull      = "full"
	ImagePricingMatrix      = "matrix"
	ImagePricingFlat        = "flat"
)

type toolPricePrefix struct {
	prefix string
	price  decimal.Decimal
}

type imageModePrefix struct {
	prefix string
	mode   string
}

// ToolPricingSnapshot is immutable after construction and can be retained for
// the full lifetime of a relay request.
type ToolPricingSnapshot struct {
	enabled            bool
	multiplierMode     string
	imagePricingMode   string
	prices             map[string]decimal.Decimal
	pricePrefixes      map[string][]toolPricePrefix
	imagePriceMatrix   map[string]map[string]decimal.Decimal
	imageModeOverrides []imageModePrefix
}

var (
	currentToolPricing atomic.Pointer[ToolPricingSnapshot]
	toolPricingMu      sync.Mutex
)

func defaultImagePriceMatrix() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"low": {
			"1024x1024": GPTImage1Low1024x1024,
			"1024x1536": GPTImage1Low1024x1536,
			"1536x1024": GPTImage1Low1536x1024,
		},
		"medium": {
			"1024x1024": GPTImage1Medium1024x1024,
			"1024x1536": GPTImage1Medium1024x1536,
			"1536x1024": GPTImage1Medium1536x1024,
		},
		"high": {
			"1024x1024": GPTImage1High1024x1024,
			"1024x1536": GPTImage1High1024x1536,
			"1536x1024": GPTImage1High1536x1024,
		},
	}
}

func cloneFloatMap(source map[string]float64) map[string]float64 {
	cloned := make(map[string]float64, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneMatrix(source map[string]map[string]float64) map[string]map[string]float64 {
	cloned := make(map[string]map[string]float64, len(source))
	for quality, sizes := range source {
		cloned[quality] = cloneFloatMap(sizes)
	}
	return cloned
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneToolPriceSetting(source ToolPriceSetting) ToolPriceSetting {
	return ToolPriceSetting{
		Enabled:                 source.Enabled,
		Prices:                  cloneFloatMap(source.Prices),
		MultiplierMode:          source.MultiplierMode,
		ImagePricingMode:        source.ImagePricingMode,
		ImagePriceMatrix:        cloneMatrix(source.ImagePriceMatrix),
		ImageModelModeOverrides: cloneStringMap(source.ImageModelModeOverrides),
	}
}

func isValidToolPrice(price float64) bool {
	return price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func validateRuleName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("工具价格名称不能为空")
	}
	if colon := strings.IndexByte(trimmed, ':'); colon >= 0 {
		if colon == 0 || strings.TrimSuffix(trimmed[colon+1:], "*") == "" {
			return fmt.Errorf("工具价格规则 %q 格式无效", name)
		}
	}
	return nil
}

func validateToolPriceSetting(setting ToolPriceSetting, tolerateLegacy bool) (ToolPriceSetting, error) {
	normalized := cloneToolPriceSetting(setting)
	if normalized.Prices == nil {
		normalized.Prices = map[string]float64{}
	}
	if normalized.ImagePriceMatrix == nil {
		normalized.ImagePriceMatrix = map[string]map[string]float64{}
	}
	if normalized.ImageModelModeOverrides == nil {
		normalized.ImageModelModeOverrides = map[string]string{}
	}
	if normalized.MultiplierMode == "" {
		normalized.MultiplierMode = ToolMultiplierGroupOnly
	}
	if normalized.ImagePricingMode == "" {
		normalized.ImagePricingMode = ImagePricingMatrix
	}

	if normalized.MultiplierMode != ToolMultiplierGroupOnly && normalized.MultiplierMode != ToolMultiplierFull {
		if !tolerateLegacy {
			return ToolPriceSetting{}, fmt.Errorf("工具倍率模式必须是 %s 或 %s", ToolMultiplierGroupOnly, ToolMultiplierFull)
		}
		common.SysError("忽略无效的工具倍率模式并使用 group_only")
		normalized.MultiplierMode = ToolMultiplierGroupOnly
	}
	if normalized.ImagePricingMode != ImagePricingMatrix && normalized.ImagePricingMode != ImagePricingFlat {
		if !tolerateLegacy {
			return ToolPriceSetting{}, fmt.Errorf("图片工具计价模式必须是 %s 或 %s", ImagePricingMatrix, ImagePricingFlat)
		}
		common.SysError("忽略无效的图片工具计价模式并使用 matrix")
		normalized.ImagePricingMode = ImagePricingMatrix
	}

	for name, price := range normalized.Prices {
		if err := validateRuleName(name); err != nil || !isValidToolPrice(price) {
			if !tolerateLegacy {
				if err != nil {
					return ToolPriceSetting{}, err
				}
				return ToolPriceSetting{}, fmt.Errorf("工具价格 %q 必须是有限的非负数", name)
			}
			common.SysError(fmt.Sprintf("忽略无效的工具价格 %q", name))
			delete(normalized.Prices, name)
		}
	}
	for quality, sizes := range normalized.ImagePriceMatrix {
		if strings.TrimSpace(quality) == "" || sizes == nil {
			if !tolerateLegacy {
				return ToolPriceSetting{}, fmt.Errorf("图片质量名称不能为空")
			}
			delete(normalized.ImagePriceMatrix, quality)
			continue
		}
		for size, price := range sizes {
			if strings.TrimSpace(size) == "" || !isValidToolPrice(price) {
				if !tolerateLegacy {
					return ToolPriceSetting{}, fmt.Errorf("图片价格 %q/%q 必须是有限的非负数", quality, size)
				}
				common.SysError(fmt.Sprintf("忽略无效的图片价格 %q/%q", quality, size))
				delete(sizes, size)
			}
		}
	}
	for prefix, mode := range normalized.ImageModelModeOverrides {
		if strings.TrimSuffix(strings.TrimSpace(prefix), "*") == "" || (mode != ImagePricingMatrix && mode != ImagePricingFlat) {
			if !tolerateLegacy {
				return ToolPriceSetting{}, fmt.Errorf("图片模型模式覆盖 %q 无效", prefix)
			}
			common.SysError(fmt.Sprintf("忽略无效的图片模型模式覆盖 %q", prefix))
			delete(normalized.ImageModelModeOverrides, prefix)
		}
	}
	return normalized, nil
}

func buildToolPricingSnapshot(setting ToolPriceSetting) *ToolPricingSnapshot {
	mergedPrices := make(map[string]float64, len(defaultToolPrices)+len(defaultToolPriceOverrides)+len(setting.Prices))
	for key, price := range defaultToolPrices {
		mergedPrices[key] = price
	}
	for key, price := range defaultToolPriceOverrides {
		mergedPrices[key] = price
	}
	for key, price := range setting.Prices {
		mergedPrices[key] = price
	}

	snapshot := &ToolPricingSnapshot{
		enabled:          setting.Enabled,
		multiplierMode:   setting.MultiplierMode,
		imagePricingMode: setting.ImagePricingMode,
		prices:           make(map[string]decimal.Decimal),
		pricePrefixes:    make(map[string][]toolPricePrefix),
		imagePriceMatrix: make(map[string]map[string]decimal.Decimal),
	}
	for key, price := range mergedPrices {
		value := decimal.NewFromFloat(price)
		colon := strings.IndexByte(key, ':')
		if colon < 0 {
			snapshot.prices[key] = value
			continue
		}
		tool := key[:colon]
		prefix := strings.TrimSuffix(key[colon+1:], "*")
		snapshot.pricePrefixes[tool] = append(snapshot.pricePrefixes[tool], toolPricePrefix{prefix: prefix, price: value})
	}
	for tool, entries := range snapshot.pricePrefixes {
		sort.Slice(entries, func(i, j int) bool {
			if len(entries[i].prefix) == len(entries[j].prefix) {
				return entries[i].prefix < entries[j].prefix
			}
			return len(entries[i].prefix) > len(entries[j].prefix)
		})
		snapshot.pricePrefixes[tool] = entries
	}

	matrix := defaultImagePriceMatrix()
	for quality, sizes := range setting.ImagePriceMatrix {
		if matrix[quality] == nil {
			matrix[quality] = map[string]float64{}
		}
		for size, price := range sizes {
			matrix[quality][size] = price
		}
	}
	for quality, sizes := range matrix {
		snapshot.imagePriceMatrix[quality] = make(map[string]decimal.Decimal, len(sizes))
		for size, price := range sizes {
			snapshot.imagePriceMatrix[quality][size] = decimal.NewFromFloat(price)
		}
	}
	for prefix, mode := range setting.ImageModelModeOverrides {
		snapshot.imageModeOverrides = append(snapshot.imageModeOverrides, imageModePrefix{
			prefix: strings.TrimSuffix(prefix, "*"),
			mode:   mode,
		})
	}
	sort.Slice(snapshot.imageModeOverrides, func(i, j int) bool {
		if len(snapshot.imageModeOverrides[i].prefix) == len(snapshot.imageModeOverrides[j].prefix) {
			return snapshot.imageModeOverrides[i].prefix < snapshot.imageModeOverrides[j].prefix
		}
		return len(snapshot.imageModeOverrides[i].prefix) > len(snapshot.imageModeOverrides[j].prefix)
	})
	return snapshot
}

// NewToolPricingSnapshot validates and freezes a standalone configuration.
// It does not replace the process-wide active setting.
func NewToolPricingSnapshot(setting ToolPriceSetting) (*ToolPricingSnapshot, error) {
	normalized, err := validateToolPriceSetting(setting, false)
	if err != nil {
		return nil, err
	}
	return buildToolPricingSnapshot(normalized), nil
}

func replaceToolPriceSetting(setting ToolPriceSetting, tolerateLegacy bool) error {
	normalized, err := validateToolPriceSetting(setting, tolerateLegacy)
	if err != nil {
		return err
	}
	snapshot := buildToolPricingSnapshot(normalized)
	toolPricingMu.Lock()
	toolPriceSetting = normalized
	currentToolPricing.Store(snapshot)
	toolPricingMu.Unlock()
	return nil
}

func CurrentToolPricingSnapshot() *ToolPricingSnapshot {
	if snapshot := currentToolPricing.Load(); snapshot != nil {
		return snapshot
	}
	RebuildToolPriceIndex()
	return currentToolPricing.Load()
}

// CurrentToolPriceSetting returns a detached copy of the persisted tool-price
// document for administrative settings APIs. Callers may safely edit it.
func CurrentToolPriceSetting() ToolPriceSetting {
	toolPricingMu.Lock()
	defer toolPricingMu.Unlock()
	return cloneToolPriceSetting(toolPriceSetting)
}

func (s *ToolPricingSnapshot) Enabled() bool { return s != nil && s.enabled }

func (s *ToolPricingSnapshot) MultiplierMode() string {
	if s == nil {
		return ToolMultiplierGroupOnly
	}
	return s.multiplierMode
}

func (s *ToolPricingSnapshot) Price(toolName, publicModel string) decimal.Decimal {
	if s == nil {
		return decimal.Zero
	}
	for _, entry := range s.pricePrefixes[toolName] {
		if publicModel != "" && strings.HasPrefix(publicModel, entry.prefix) {
			return entry.price
		}
	}
	return s.prices[toolName]
}

func (s *ToolPricingSnapshot) matrixPrice(quality, size string) decimal.Decimal {
	if s != nil {
		if sizes := s.imagePriceMatrix[quality]; sizes != nil {
			if price, ok := sizes[size]; ok {
				return price
			}
		}
		if sizes := s.imagePriceMatrix["high"]; sizes != nil {
			if price, ok := sizes["1024x1024"]; ok {
				return price
			}
		}
	}
	return decimal.NewFromFloat(GPTImage1High1024x1024)
}

// ImagePrice returns a per-call price in matrix mode and a per-1000-calls
// price in flat mode. Callers must use the returned mode when calculating.
func (s *ToolPricingSnapshot) ImagePrice(publicModel, quality, size string) (decimal.Decimal, string) {
	mode := ImagePricingMatrix
	if s != nil {
		mode = s.imagePricingMode
		for _, override := range s.imageModeOverrides {
			if publicModel != "" && strings.HasPrefix(publicModel, override.prefix) {
				mode = override.mode
				break
			}
		}
	}
	if mode == ImagePricingFlat {
		return s.Price("image_generation", publicModel), mode
	}
	return s.matrixPrice(quality, size), mode
}

func decodeStrictJSON(value string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("配置包含多余内容")
	}
	return nil
}

func ApplyToolPriceSetting(value string) error {
	var setting ToolPriceSetting
	if err := decodeStrictJSON(value, &setting); err != nil {
		return fmt.Errorf("解析工具价格配置失败: %w", err)
	}
	return replaceToolPriceSetting(setting, false)
}

func ValidateToolPriceOption(key, value string) error {
	switch key {
	case "enabled":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("工具计费开关必须是布尔值")
		}
		return nil
	case "prices":
		var prices map[string]float64
		if err := decodeStrictJSON(value, &prices); err != nil || prices == nil {
			return fmt.Errorf("工具价格必须是 JSON 对象")
		}
		_, err := validateToolPriceSetting(ToolPriceSetting{
			Prices: prices, MultiplierMode: ToolMultiplierGroupOnly, ImagePricingMode: ImagePricingMatrix,
		}, false)
		return err
	case "multiplier_mode":
		if value != ToolMultiplierGroupOnly && value != ToolMultiplierFull {
			return fmt.Errorf("工具倍率模式无效")
		}
		return nil
	case "image_pricing_mode":
		if value != ImagePricingMatrix && value != ImagePricingFlat {
			return fmt.Errorf("图片工具计价模式无效")
		}
		return nil
	case "image_price_matrix":
		var matrix map[string]map[string]float64
		if err := decodeStrictJSON(value, &matrix); err != nil || matrix == nil {
			return fmt.Errorf("图片价格矩阵必须是 JSON 对象")
		}
		_, err := validateToolPriceSetting(ToolPriceSetting{
			Prices: map[string]float64{}, MultiplierMode: ToolMultiplierGroupOnly,
			ImagePricingMode: ImagePricingMatrix, ImagePriceMatrix: matrix,
		}, false)
		return err
	case "image_model_mode_overrides":
		var overrides map[string]string
		if err := decodeStrictJSON(value, &overrides); err != nil || overrides == nil {
			return fmt.Errorf("图片模型模式覆盖必须是 JSON 对象")
		}
		_, err := validateToolPriceSetting(ToolPriceSetting{
			Prices: map[string]float64{}, MultiplierMode: ToolMultiplierGroupOnly,
			ImagePricingMode: ImagePricingMatrix, ImageModelModeOverrides: overrides,
		}, false)
		return err
	default:
		return fmt.Errorf("未知的工具价格配置项 %q", key)
	}
}

func ApplyToolPriceOption(key, value string) error {
	if err := ValidateToolPriceOption(key, value); err != nil {
		return err
	}
	toolPricingMu.Lock()
	next := cloneToolPriceSetting(toolPriceSetting)
	toolPricingMu.Unlock()
	switch key {
	case "enabled":
		next.Enabled, _ = strconv.ParseBool(value)
	case "prices":
		_ = json.Unmarshal([]byte(value), &next.Prices)
	case "multiplier_mode":
		next.MultiplierMode = value
	case "image_pricing_mode":
		next.ImagePricingMode = value
	case "image_price_matrix":
		_ = json.Unmarshal([]byte(value), &next.ImagePriceMatrix)
	case "image_model_mode_overrides":
		_ = json.Unmarshal([]byte(value), &next.ImageModelModeOverrides)
	}
	return replaceToolPriceSetting(next, false)
}
