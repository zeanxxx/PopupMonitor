package main

import (
	_ "embed"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
)

var (
	//go:embed assets/active.ico
	activeIcon []byte
	//go:embed assets/inactive.ico
	inactiveIcon    []byte
	procKeybdEvent  = user32.NewProc("keybd_event")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

const (
	VK_CONTROL      = 0x11
	VK_LWIN         = 0x5B
	VK_L            = 0x4C
	KEYEVENTF_KEYUP = 0x0002
	MB_OK           = 0x00000000
)

func onReady() {

	// inactiveIcon, _ = os.ReadFile("assets/inactive.ico")
	systray.SetTitle("PopUp Monitor")
	setActiveTray()
	mOpen := systray.AddMenuItem("Open LiveCaptions", "open live captions window")
	systray.AddSeparator()
	mStart := systray.AddMenuItem("Start Monitoring", "start monitoring popups")
	mStop := systray.AddMenuItem("Stop Monitoring", "stop monitoring popups")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem("About", "about this application")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Exit", "exit application")

	go func() {
		openLiveCaptionsWindow()
		monitorPopupsOnce()
		startProcessWatcher()
		startMonitorPopups()
		for {
			select {
			case <-mOpen.ClickedCh:
				fmt.Println("Open LiveCaptions window")
				openLiveCaptionsWindow()
			case <-mStart.ClickedCh:
				setActiveTray()
				startMonitorPopups()
				fmt.Println("Started monitoring popups")
			case <-mStop.ClickedCh:
				setInactiveTray()
				stopMonitoringPopups()
				fmt.Println("Stopping monitorning popups")
			case <-mAbout.ClickedCh:
				showAbout()
			case <-mQuit.ClickedCh:
				systray.Quit()
				os.Exit(0)
			}
		}
	}()
}

func onExit() {
	// clean up here
}
func setInactiveTray() {
	systray.SetIcon(inactiveIcon)
	systray.SetTooltip("PopUp Monitor (Stopped)")
}

func setActiveTray() {
	systray.SetIcon(activeIcon)
	systray.SetTooltip("PopUp Monitor (Running)")
}

func openLiveCaptionsWindow() {
	procKeybdEvent.Call(uintptr(VK_CONTROL), 0, 0, 0)
	procKeybdEvent.Call(uintptr(VK_LWIN), 0, 0, 0)
	procKeybdEvent.Call(uintptr(VK_L), 0, 0, 0)
	procKeybdEvent.Call(uintptr(VK_L), 0, KEYEVENTF_KEYUP, 0)
	procKeybdEvent.Call(uintptr(VK_LWIN), 0, KEYEVENTF_KEYUP, 0)
	procKeybdEvent.Call(uintptr(VK_CONTROL), 0, KEYEVENTF_KEYUP, 0)
}

func showAbout() {
	aboutText :=
		"About PopUp Monitor\n\n" +
			"This app monitors and automatically hides Windows Live Captions popups.\n\n" +
			"Key Features:\n" +
			"• Runs in system tray\n" +
			"• Start/Stop monitoring\n" +
			"• Automatic popup detection\n" +
			"• Popup hide notifications\n" +
			"To interact with Live Captions, stop monitoring first.\n" +
			"Enjoy!\n\n" +
			"Feedback: zeansss@outlook.com\n" +
			"By Zean"
	showMessage("About PopUp Monitor", aboutText)
}

func showMessage(title, text string) {

	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		MB_OK,
	)
}
