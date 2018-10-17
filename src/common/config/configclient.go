package config

// Client used to retrieve configuration
type Client interface {
	GetSettingByGroup(groupName string) ([]Value, error)
	GetSettingByScope(scope string) ([]Value, error)
	GetSetting(keyName string) (Value, error)
	UpdateConfig(cfg map[string]string) error
	UpdateConfigValue(key string, value string) error
}
