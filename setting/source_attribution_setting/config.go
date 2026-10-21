package source_attribution_setting

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	EnabledKey                    = "source_attribution.enabled"
	TokenAPIEnforcementEnabledKey = "source_attribution.token_api_enforcement_enabled"
	OptionsKey                    = "source_attribution.options"

	MaxSourceDescriptionLength = 200
)

type Option struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	TargetUserID        int    `json:"target_user_id"`
	TargetAffCode       string `json:"target_aff_code,omitempty"`
	RequiresDescription bool   `json:"requires_description"`
	SortOrder           int    `json:"sort_order"`
}

type PublicOption struct {
	Id                  string `json:"id"`
	Name                string `json:"name"`
	RequiresDescription bool   `json:"requires_description"`
	SortOrder           int    `json:"sort_order"`
}

type Settings struct {
	Enabled                    bool     `json:"enabled"`
	TokenAPIEnforcementEnabled bool     `json:"token_api_enforcement_enabled"`
	Options                    []Option `json:"options"`
}

func DefaultOptions() map[string]string {
	return map[string]string{
		EnabledKey:                    "false",
		TokenAPIEnforcementEnabledKey: "false",
		OptionsKey:                    "[]",
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	settings := Settings{
		Enabled:                    parseBoolOption(common.OptionMap[EnabledKey]),
		TokenAPIEnforcementEnabled: parseBoolOption(common.OptionMap[TokenAPIEnforcementEnabledKey]),
	}
	if err := common.UnmarshalJsonStr(common.OptionMap[OptionsKey], &settings.Options); err != nil {
		settings.Options = nil
	}
	return NormalizeSettings(settings)
}

func NormalizeSettings(settings Settings) Settings {
	for i := range settings.Options {
		settings.Options[i].Id = strings.TrimSpace(settings.Options[i].Id)
		settings.Options[i].Name = strings.TrimSpace(settings.Options[i].Name)
		settings.Options[i].TargetAffCode = strings.TrimSpace(settings.Options[i].TargetAffCode)
	}
	sort.SliceStable(settings.Options, func(i, j int) bool {
		if settings.Options[i].SortOrder == settings.Options[j].SortOrder {
			return settings.Options[i].Id < settings.Options[j].Id
		}
		return settings.Options[i].SortOrder < settings.Options[j].SortOrder
	})
	return settings
}

func ValidateSettings(settings Settings) error {
	settings = NormalizeSettings(settings)
	if !settings.Enabled {
		return nil
	}
	if len(settings.Options) == 0 {
		return errors.New("启用来源归属时选项不能为空")
	}
	seen := make(map[string]struct{}, len(settings.Options))
	for _, option := range settings.Options {
		if option.Id == "" {
			return errors.New("来源归属选项 ID 不能为空")
		}
		if option.Name == "" {
			return errors.New("来源归属选项名称不能为空")
		}
		if option.TargetUserID <= 0 && option.TargetAffCode == "" {
			return errors.New("来源归属选项目标用户 ID 不能为空")
		}
		if _, ok := seen[option.Id]; ok {
			return errors.New("来源归属选项 ID 不能重复")
		}
		seen[option.Id] = struct{}{}
	}
	return nil
}

func SettingsToOptions(settings Settings) (map[string]string, error) {
	settings = NormalizeSettings(settings)
	optionsBytes, err := common.Marshal(settings.Options)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		EnabledKey:                    strconv.FormatBool(settings.Enabled),
		TokenAPIEnforcementEnabledKey: strconv.FormatBool(settings.TokenAPIEnforcementEnabled),
		OptionsKey:                    string(optionsBytes),
	}, nil
}

func PublicOptions(options []Option) []PublicOption {
	publicOptions := make([]PublicOption, 0, len(options))
	for _, option := range NormalizeSettings(Settings{Options: options}).Options {
		publicOptions = append(publicOptions, PublicOption{
			Id:                  option.Id,
			Name:                option.Name,
			RequiresDescription: option.RequiresDescription,
			SortOrder:           option.SortOrder,
		})
	}
	return publicOptions
}

func FindOption(settings Settings, optionId string) (Option, bool) {
	optionId = strings.TrimSpace(optionId)
	for _, option := range NormalizeSettings(settings).Options {
		if option.Id == optionId {
			return option, true
		}
	}
	return Option{}, false
}

func parseBoolOption(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
