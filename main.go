package main

import (
	"pop-up-monitor/lang"
	"syscall"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
)

func main() {
	// check if the mutex already exists
	name, _ := syscall.UTF16PtrFromString("Global\\PopUpMonitorMutex")
	_, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		//msg := fmt.Sprintf("CreateMutex fail:", err)
		//showMessage("Warning", msg)
		showMessage("Information", "The program is already running.")
		return

		//os.Exit(1)
	}
	if windows.GetLastError() == syscall.ERROR_ALREADY_EXISTS {
		showMessage("Warning", "The program is already running.")
		return
		//os.Exit(0)
	}
	lang.DetectSystemLanguage()
	lang.LoadTranslations()

	systray.Run(onReady, onExit)

}
