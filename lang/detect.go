package lang

import (
	"syscall"
)

var CurrentLang = "en-US"

func DetectSystemLanguage() {
	langID := getSystemLangID()
	switch langID {
	case 0x404:
		CurrentLang = "zh-TW"
	case 0x804:
		CurrentLang = "zh-CN"
	default:
		CurrentLang = "en-US"
	}
}

func getSystemLangID() uint16 {
	modkernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetUserDefaultUILanguage := modkernel32.NewProc("GetUserDefaultUILanguage")
	ret, _, _ := procGetUserDefaultUILanguage.Call()
	return uint16(ret)
}
