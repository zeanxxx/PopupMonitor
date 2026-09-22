package main

import (
	"pop-up-monitor/lang"
	"syscall"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
)

func main() {
	_ = enableDPIAwareness()
	lang.DetectSystemLanguage()
	lang.LoadTranslations()

	name, err := windows.UTF16PtrFromString("Global\\PopUpMonitorMutex")
	if err != nil {
		return
	}

	mutex, err := windows.CreateMutex(nil, false, name)
	if err == syscall.ERROR_ALREADY_EXISTS {
		if mutex != 0 {
			windows.CloseHandle(mutex)
		}
		showMessage(lang.AlreadyRunningTitle(), lang.AlreadyRunningMessage())
		return
	}
	if err != nil {
		showMessage(lang.MutexErrorTitle(), lang.MutexErrorMessage())
		return
	}

	defer windows.CloseHandle(mutex)

	systray.Run(onReady, onExit)
}
