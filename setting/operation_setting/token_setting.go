package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const AllowCustomKeysOptionKey = "token_setting.allow_custom_keys"

// TokenSetting 令牌相关配置
type TokenSetting struct {
	MaxUserTokens   int  `json:"max_user_tokens"`   // 每用户最大令牌数量
	AllowCustomKeys bool `json:"allow_custom_keys"` // 是否允许普通用户创建自定义 Key
}

// 默认配置
var tokenSetting = TokenSetting{
	MaxUserTokens:   1000, // 默认每用户最多 1000 个令牌
	AllowCustomKeys: true,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("token_setting", &tokenSetting)
}

// GetTokenSetting 获取令牌配置
func GetTokenSetting() *TokenSetting {
	return &tokenSetting
}

// GetMaxUserTokens 获取每用户最大令牌数量
func GetMaxUserTokens() int {
	return GetTokenSetting().MaxUserTokens
}
