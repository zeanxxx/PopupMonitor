package main

import (
	"fmt"
	"pop-up-monitor/lang"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procShowWindow = user32.NewProc("ShowWindow")
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	psapi          = syscall.NewLazyDLL("psapi.dll")

	procEnumWindows        = user32.NewProc("EnumWindows")
	procOpenProcess        = kernel32.NewProc("OpenProcess")
	procCloseHandle        = kernel32.NewProc("CloseHandle")
	procGetModuleBaseNameW = psapi.NewProc("GetModuleBaseNameW")
	procGetWindowTextW     = user32.NewProc("GetWindowTextW")
	procGetWindowTextLenW  = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW      = user32.NewProc("GetClassNameW")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")

	procSetWinEventHook  = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent   = user32.NewProc("UnhookWinEvent")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	//procTranslateMessage = user32.NewProc("TranslateMessage")
)

const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
	SW_HIDE                   = 0

	EVENT_OBJECT_CREATE   = 0x8000
	EVENT_OBJECT_DESTROY  = 0x8001
	EVENT_OBJECT_SHOW     = 0x8002
	WINEVENT_OUTOFCONTEXT = 0x0000
)

type HOOK uintptr
type HWND uintptr
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// 定義回呼函式型別
type WinEventProc func(hWinEventHook HOOK, event uint32, hwnd HWND, idObject, idChild int32, idEventThread, dwmsEventTime uint32)

var (
	hookHandle                 uintptr
	hookActive                 = true
	stopMonitoringChan         chan struct{}
	popup_winEventProcCallback = syscall.NewCallback(func(hWinEventHook HOOK, event uint32, hwnd HWND, idObject, idChild int32, idEventThread, dwmsEventTime uint32) uintptr {
		if !hookActive {
			return 0
		}
		handlePopupsByHwnd(uintptr(hwnd))
		return 0
	})

	process_winEventProcCallback = syscall.NewCallback(func(hWinEventHook HOOK, event uint32, hwnd HWND, idObject, idChild int32, idEventThread, dwmsEventTime uint32) uintptr {

		handleProcessByHwnd(uintptr(hwnd), event)
		return 0
	})
)

func closeCurrentPopup(hwnd uintptr) {
	if hwnd != 0 {
		procShowWindow.Call(hwnd, SW_HIDE)
	}
}

func enumWindows(callback func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if callback(hwnd) {
			return 1
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}

func getProcessName(hwnd uintptr) (string, error) {
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	if pid == 0 {
		return "", fmt.Errorf("Invalid PID")
	}

	hProcess, _, _ := procOpenProcess.Call(
		PROCESS_QUERY_INFORMATION|PROCESS_VM_READ,
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return "", fmt.Errorf("Unable to open the process")
	}
	defer procCloseHandle.Call(hProcess)

	buf := make([]uint16, 260)
	procGetModuleBaseNameW.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)

	return syscall.UTF16ToString(buf), nil
}
func getWindowText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLenW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)+1))
	return syscall.UTF16ToString(buf)
}

func getClassName(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
func handlePopupsByHwnd(hwnd uintptr) {
	//hwnd := findWindow("Xaml_WindowedPopupClass", "PopupHost")
	if hwnd != 0 {
		className := getClassName(hwnd)
		title := getWindowText(hwnd)
		if className == lang.PopupClassName() && title == lang.PopupTitle() {
			fmt.Printf("className %s\n", className)
			fmt.Printf("title %s\n", title)
			processName, _ := getProcessName(hwnd)
			if processName == lang.PopupProcessName() {
				fmt.Printf("Closing LiveCaptions window: %s\n", hwnd)
				closeCurrentPopup(hwnd)

			} else {
				//fmt.Printf("Found popup from process: %s, but not closing it\n", processName)
			}
		}
	} else {
		fmt.Println("LiveCaptions window not found")
	}
}

func handleProcessByHwnd(hwnd uintptr, event uint32) {

	if hwnd != 0 {
		className := getClassName(hwnd)

		title := getWindowText(hwnd)
		if len(title) > 0 {
			if className == lang.PropClassName() && title == lang.PropTitle() {
				fmt.Printf("[Process]className %s\n", className)
				fmt.Printf("[Process]title %s\n", title)
				processName, _ := getProcessName(hwnd)
				if processName == lang.PopupProcessName() {
					fmt.Printf("LiveCaptions window action: %s\n", hwnd)
					switch event {
					case EVENT_OBJECT_SHOW:
						fmt.Printf("SHOW: hwnd=0x%x\n", hwnd)
						setActiveTray()
						startMonitorPopups()
					case EVENT_OBJECT_CREATE:
						fmt.Printf("CREATE: hwnd=0x%x\n", hwnd)
						setActiveTray()
						startMonitorPopups()
					case EVENT_OBJECT_DESTROY:
						fmt.Printf("DESTROY: hwnd=0x%x\n", hwnd)
						setInactiveTray()
						stopMonitoringPopups()
					default:
						fmt.Printf("other events %d, hwnd=0x%x\n", event, hwnd)
					}
				} else {
					fmt.Printf("Found popup from process: %s, but not closing it\n", processName)
				}
			}
		}

	} else {
		fmt.Println("LiveCaptions window not found")
	}
}

func monitorPopupsOnce() {
	enumWindows(func(hwnd uintptr) bool {
		handlePopupsByHwnd(hwnd)
		return true
	})
}

// use WinEvent hook to monitor popup windows is opened or not
func startProcessWatcher() {
	go func() {
		hWinEventHook, _, _ := procSetWinEventHook.Call(
			uintptr(EVENT_OBJECT_CREATE),
			uintptr(EVENT_OBJECT_DESTROY),
			0,
			process_winEventProcCallback,
			0,
			0,
			uintptr(WINEVENT_OUTOFCONTEXT),
		)
		if hWinEventHook == 0 {
			fmt.Println("[ProcessWatch] Failed to set WinEvent hook")
			return
		}
		defer procUnhookWinEvent.Call(hWinEventHook)

		fmt.Println("[ProcessWatch] Hook set successfully")

		var msg MSG
		for {

			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(ret) == -1 {
				break
			} else if ret == 0 {
				return
			}
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

func startMonitorPopups() {
	if hookHandle != 0 {
		fmt.Println("Already exist hook")

		return
	}
	stopMonitoringChan = make(chan struct{})
	go func() {

		// init COM (STA)
		ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
		defer ole.CoUninitialize()
		hWinEventHook, _, _ := procSetWinEventHook.Call(
			uintptr(EVENT_OBJECT_CREATE), // eventMin
			uintptr(EVENT_OBJECT_SHOW),   // eventMax
			0,
			popup_winEventProcCallback,
			0,
			0,
			uintptr(WINEVENT_OUTOFCONTEXT),
		)

		if hWinEventHook == 0 {
			fmt.Println("[PopupMonitor]Failed to set WinEvent hook")
			return
		} else {
			fmt.Println("[PopupMonitor]WinEvent hook set successfully")
		}
		hookHandle = hWinEventHook
		hookActive = true
		//defer procUnhookWinEvent.Call(hWinEventHook)

		var msg MSG
		for {
			select {
			case <-stopMonitoringChan:
				return
			default:
				ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
				if int32(ret) == -1 {
					break
				}
				//procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
			}
		}
	}()
}

func stopMonitoringPopups() {

	if hookHandle != 0 {
		procUnhookWinEvent.Call(hookHandle)
		hookHandle = 0
		hookActive = false
		close(stopMonitoringChan)
		stopMonitoringChan = make(chan struct{})
	}
}
