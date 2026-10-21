package compliance_setting

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	StatementEnabledKey           = "compliance.statement_enabled"
	StatementVersionKey           = "compliance.statement_version"
	StatementContentKey           = "compliance.statement_content"
	StatementItemsKey             = "compliance.statement_items"
	StatementDisplayModeKey       = "compliance.statement_display_mode"
	TokenAPIEnforcementEnabledKey = "compliance.token_api_enforcement_enabled"
	ShowVersionToUserKey          = "compliance.show_version_to_user"
	ShowContentHashToUserKey      = "compliance.show_content_hash_to_user"

	DisplayModeSingle   = "single"
	DisplayModeMultiple = "multiple"

	SourceTypeManual        = "manual"
	SourceTypeUserAgreement = "user_agreement"
	SourceTypePrivacyPolicy = "privacy_policy"

	ConfirmationModeCheckbox = "checkbox"
	ConfirmationModePhrase   = "phrase"
	ConfirmationModeSegments = "segments"
)

type StatementItem struct {
	ItemKey            string   `json:"item_key"`
	Title              string   `json:"title"`
	SourceType         string   `json:"source_type"`
	Version            string   `json:"version"`
	Content            string   `json:"content"`
	SortOrder          int      `json:"sort_order"`
	ConfirmationMode   string   `json:"confirmation_mode"`
	ConfirmationPhrase string   `json:"confirmation_phrase"`
	Segments           []string `json:"segments"`
}

type Settings struct {
	StatementEnabled           bool            `json:"statement_enabled"`
	StatementVersion           string          `json:"statement_version"`
	StatementContent           string          `json:"statement_content"`
	StatementItems             []StatementItem `json:"statement_items"`
	StatementDisplayMode       string          `json:"statement_display_mode"`
	TokenAPIEnforcementEnabled bool            `json:"token_api_enforcement_enabled"`
	ShowVersionToUser          bool            `json:"show_version_to_user"`
	ShowContentHashToUser      bool            `json:"show_content_hash_to_user"`
}

func DefaultOptions() map[string]string {
	return map[string]string{
		StatementEnabledKey:           "false",
		StatementVersionKey:           "",
		StatementContentKey:           "",
		StatementItemsKey:             "[]",
		StatementDisplayModeKey:       DisplayModeSingle,
		TokenAPIEnforcementEnabledKey: "false",
		ShowVersionToUserKey:          "false",
		ShowContentHashToUserKey:      "false",
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	settings := Settings{
		StatementEnabled:           parseBoolOption(common.OptionMap[StatementEnabledKey]),
		StatementVersion:           common.OptionMap[StatementVersionKey],
		StatementContent:           common.OptionMap[StatementContentKey],
		StatementDisplayMode:       common.OptionMap[StatementDisplayModeKey],
		TokenAPIEnforcementEnabled: parseBoolOption(common.OptionMap[TokenAPIEnforcementEnabledKey]),
		ShowVersionToUser:          parseBoolOption(common.OptionMap[ShowVersionToUserKey]),
		ShowContentHashToUser:      parseBoolOption(common.OptionMap[ShowContentHashToUserKey]),
	}
	if err := common.UnmarshalJsonStr(common.OptionMap[StatementItemsKey], &settings.StatementItems); err != nil {
		settings.StatementItems = nil
	}
	return NormalizeSettings(settings)
}

func NormalizeSettings(settings Settings) Settings {
	settings.StatementVersion = strings.TrimSpace(settings.StatementVersion)
	settings.StatementDisplayMode = strings.TrimSpace(settings.StatementDisplayMode)
	if settings.StatementDisplayMode != DisplayModeMultiple {
		settings.StatementDisplayMode = DisplayModeSingle
	}
	for i := range settings.StatementItems {
		settings.StatementItems[i].ItemKey = strings.TrimSpace(settings.StatementItems[i].ItemKey)
		settings.StatementItems[i].Title = strings.TrimSpace(settings.StatementItems[i].Title)
		settings.StatementItems[i].SourceType = strings.TrimSpace(settings.StatementItems[i].SourceType)
		settings.StatementItems[i].Version = strings.TrimSpace(settings.StatementItems[i].Version)
		settings.StatementItems[i].ConfirmationMode = strings.TrimSpace(settings.StatementItems[i].ConfirmationMode)
		settings.StatementItems[i].ConfirmationPhrase = strings.TrimSpace(settings.StatementItems[i].ConfirmationPhrase)
		if settings.StatementItems[i].SourceType == "" {
			settings.StatementItems[i].SourceType = SourceTypeManual
		}
		if settings.StatementItems[i].ConfirmationMode == "" {
			settings.StatementItems[i].ConfirmationMode = ConfirmationModeCheckbox
		}
	}
	sort.SliceStable(settings.StatementItems, func(i, j int) bool {
		if settings.StatementItems[i].SortOrder == settings.StatementItems[j].SortOrder {
			return settings.StatementItems[i].ItemKey < settings.StatementItems[j].ItemKey
		}
		return settings.StatementItems[i].SortOrder < settings.StatementItems[j].SortOrder
	})
	return settings
}

func ValidateSettings(settings Settings) error {
	settings = NormalizeSettings(settings)
	if !settings.StatementEnabled {
		return nil
	}
	items := EffectiveStatementItems(settings)
	if len(items) == 0 {
		if settings.StatementVersion == "" {
			return errors.New("合规声明版本不能为空")
		}
		return errors.New("合规声明内容不能为空")
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.ItemKey == "" {
			return errors.New("合规声明条款标识不能为空")
		}
		if _, ok := seen[item.ItemKey]; ok {
			return errors.New("合规声明条款标识不能重复")
		}
		seen[item.ItemKey] = struct{}{}
		if item.Version == "" {
			return errors.New("合规声明版本不能为空")
		}
		if strings.TrimSpace(item.Content) == "" {
			return errors.New("合规声明内容不能为空")
		}
		switch item.SourceType {
		case SourceTypeManual, SourceTypeUserAgreement, SourceTypePrivacyPolicy:
		default:
			return errors.New("无效的合规声明来源类型")
		}
		switch item.ConfirmationMode {
		case ConfirmationModeCheckbox:
		case ConfirmationModePhrase:
			if item.ConfirmationPhrase == "" {
				return errors.New("确认短语不能为空")
			}
		case ConfirmationModeSegments:
			if len(item.Segments) == 0 {
				return errors.New("分段确认内容不能为空")
			}
		default:
			return errors.New("无效的合规确认方式")
		}
	}
	return nil
}

func EffectiveStatementItems(settings Settings) []StatementItem {
	settings = NormalizeSettings(settings)
	if len(settings.StatementItems) > 0 {
		return resolveStatementItemContent(settings.StatementItems)
	}
	if settings.StatementVersion == "" && strings.TrimSpace(settings.StatementContent) == "" {
		return nil
	}
	return []StatementItem{
		{
			ItemKey:          "statement",
			Title:            "合规声明",
			SourceType:       SourceTypeManual,
			Version:          settings.StatementVersion,
			Content:          settings.StatementContent,
			SortOrder:        1,
			ConfirmationMode: ConfirmationModeCheckbox,
		},
	}
}

func UsesSourceType(settings Settings, sourceType string) bool {
	settings = NormalizeSettings(settings)
	for _, item := range settings.StatementItems {
		if item.SourceType == sourceType {
			return true
		}
	}
	return false
}

func resolveStatementItemContent(items []StatementItem) []StatementItem {
	resolved := make([]StatementItem, len(items))
	copy(resolved, items)
	legalSettings := system_setting.GetLegalSettings()
	for i := range resolved {
		switch resolved[i].SourceType {
		case SourceTypeUserAgreement:
			resolved[i].Content = legalSettings.UserAgreement
		case SourceTypePrivacyPolicy:
			resolved[i].Content = legalSettings.PrivacyPolicy
		}
	}
	return resolved
}

func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func SettingsToOptions(settings Settings) map[string]string {
	settings = NormalizeSettings(settings)
	statementItemsBytes, err := common.Marshal(settings.StatementItems)
	if err != nil {
		statementItemsBytes = []byte("[]")
	}
	return map[string]string{
		StatementEnabledKey:           strconv.FormatBool(settings.StatementEnabled),
		StatementVersionKey:           settings.StatementVersion,
		StatementContentKey:           settings.StatementContent,
		StatementItemsKey:             string(statementItemsBytes),
		StatementDisplayModeKey:       settings.StatementDisplayMode,
		TokenAPIEnforcementEnabledKey: strconv.FormatBool(settings.TokenAPIEnforcementEnabled),
		ShowVersionToUserKey:          strconv.FormatBool(settings.ShowVersionToUser),
		ShowContentHashToUserKey:      strconv.FormatBool(settings.ShowContentHashToUser),
	}
}

func parseBoolOption(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
