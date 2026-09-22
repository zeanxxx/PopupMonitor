package main

import (
	_ "embed"
	"fmt"
	"os"
	"pop-up-monitor/lang"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
)

var (
	//go:embed assets/active.ico
	activeIcon []byte
	//go:embed assets/inactive.ico
	inactiveIcon                      []byte
	procKeybdEvent                    = user32.NewProc("keybd_event")
	procMessageBoxW                   = user32.NewProc("MessageBoxW")
	procShellExecuteW                 = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	VK_CONTROL      = 0x11
	VK_LWIN         = 0x5B
	VK_L            = 0x4C
	KEYEVENTF_KEYUP = 0x0002
	MB_OK           = 0x00000000

	MB_OKCANCEL     = 0x00000001
	MB_ICONQUESTION = 0x00000020
	MB_DEFBUTTON2   = 0x00000100
	IDOK            = 1
)
const feedbackURL = "https://github.com/zeanxxx/PopupMonitor/issues"

func enableDPIAwareness() error {
	const DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3)

	ret, _, err := procSetProcessDpiAwarenessContext.Call(
		DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2,
	)
	if ret == 0 {
		return fmt.Errorf("failed to enable DPI awareness: %w", err)
	}
	return nil
}

func onReady() {

	// inactiveIcon, _ = os.ReadFile("assets/inactive.ico")
	systray.SetTitle("PopUp Monitor")
	setActiveTray()
	mOpen := systray.AddMenuItem(lang.MenuOpen(), lang.MenuOpenTip())
	systray.AddSeparator()
	mStart := systray.AddMenuItem(lang.MenuStart(), lang.MenuStartTip())
	mStop := systray.AddMenuItem(lang.MenuStop(), lang.MenuStopTip())
	systray.AddSeparator()
	mAbout := systray.AddMenuItem(lang.MenuAbout(), lang.MenuAboutTip())
	mFeedback := systray.AddMenuItem(lang.MenuFeedback(), lang.MenuFeedbackTip())
	systray.AddSeparator()
	mQuit := systray.AddMenuItem(lang.MenuExit(), lang.MenuExitTip())

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

			case <-mFeedback.ClickedCh:
				confirmExternalLink(feedbackURL)
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
	systray.SetTooltip(lang.TrayStopped())
}

func setActiveTray() {
	systray.SetIcon(activeIcon)
	systray.SetTooltip(lang.TrayRunning())
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
	aboutTitle := lang.MenuAboutTip()
	aboutText := lang.AboutContent()
	showMessage(aboutTitle, aboutText)
}

func confirmExternalLink(url string) {
	title, _ := syscall.UTF16PtrFromString(lang.ExternalLinkTitle())
	content, _ := syscall.UTF16PtrFromString(lang.ExternalLinkMessage() + "\n\n" + url)

	ret, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(content)),
		uintptr(unsafe.Pointer(title)),
		MB_OKCANCEL|MB_ICONQUESTION|MB_DEFBUTTON2,
	)

	if ret == IDOK {
		openBrowser(url)
	}
}

func openBrowser(url string) {
	u, _ := syscall.UTF16PtrFromString(url)
	v, _ := syscall.UTF16PtrFromString("open")

	procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(u)),
		0,
		0,
		1,
	)
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
