package lang

import (
	_ "embed"
	"encoding/json"
)

var translations map[string]map[string]string

//go:embed locales.json
var localeData []byte

func LoadTranslations() {
	json.Unmarshal(localeData, &translations)
}

func get(key string) string {
	if val, ok := translations[CurrentLang][key]; ok {
		return val
	}
	return key // fallback
}
