package main

import (
	"fmt"
	"pop-up-monitor/lang"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
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

	procSetWinEventHook    = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent     = user32.NewProc("UnhookWinEvent")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
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

	WINEVENT_SKIPOWNPROCESS = 0x0002
	WM_QUIT                 = 0x0012

	OBJID_WINDOW = 0
	CHILDID_SELF = 0
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

var (
	liveCaptionsHWND uintptr
	hookHandle       uintptr

	monitorMu       sync.Mutex
	monitorThreadID uintptr
	monitorDone     chan struct{}
	monitorStopping bool

	popup_winEventProcCallback = syscall.NewCallback(func(
		hWinEventHook HOOK,
		event uint32,
		hwnd HWND,
		idObject, idChild int32,
		idEventThread, dwmsEventTime uint32,
	) uintptr {
		handlePopupsByHwnd(uintptr(hwnd))
		return 0
	})

	process_winEventProcCallback = syscall.NewCallback(func(
		hWinEventHook HOOK,
		event uint32,
		hwnd HWND,
		idObject, idChild int32,
		idEventThread, dwmsEventTime uint32,
	) uintptr {
		if hwnd == 0 ||
			idObject != OBJID_WINDOW ||
			idChild != CHILDID_SELF {
			return 0
		}

		handleProcessByHwnd(uintptr(hwnd), event)
		return 0
	})
)

func closeCurrentPopup(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	procShowWindow.Call(hwnd, SW_HIDE)
	recordPopupBlocked()
}

func enumWindows(callback func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if callback(hwnd) {
			return 1
		}
		return 0
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
	procGetWindowTextW.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	return syscall.UTF16ToString(buf)
}

func getClassName(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func handlePopupsByHwnd(hwnd uintptr) {
	if hwnd == 0 {
		return
	}

	className := getClassName(hwnd)
	if className != lang.PopupClassName() {
		return
	}

	title := getWindowText(hwnd)
	if title != lang.PopupTitle() {
		return
	}

	processName, err := getProcessName(hwnd)
	if err != nil || processName != lang.PopupProcessName() {
		return
	}
	fmt.Printf("className %s\n", className)
	fmt.Printf("title %s\n", title)
	fmt.Printf("Closing LiveCaptions window: %v\n", hwnd)
	closeCurrentPopup(hwnd)
}

func handleProcessByHwnd(hwnd uintptr, event uint32) {
	if hwnd == 0 {
		return
	}
	// handle destory first
	if event == EVENT_OBJECT_DESTROY {
		if liveCaptionsHWND == 0 || hwnd != liveCaptionsHWND {
			return
		}

		fmt.Printf("[Process] DESTROY: hwnd=0x%x\n", hwnd)

		liveCaptionsHWND = 0
		setInactiveTray()
		stopMonitoringPopups()
		return
	}

	if event != EVENT_OBJECT_CREATE && event != EVENT_OBJECT_SHOW {
		return
	}

	if getClassName(hwnd) != lang.PropClassName() {
		return
	}

	if getWindowText(hwnd) != lang.PropTitle() {
		return
	}

	processName, err := getProcessName(hwnd)
	if err != nil || processName != lang.PopupProcessName() {
		return
	}

	liveCaptionsHWND = hwnd

	fmt.Printf("[Process] CREATE/SHOW: hwnd=0x%x\n", hwnd)
	setActiveTray()
	startMonitorPopups()
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
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		hWinEventHook, _, _ := procSetWinEventHook.Call(
			uintptr(EVENT_OBJECT_CREATE),
			uintptr(EVENT_OBJECT_SHOW),
			0,
			process_winEventProcCallback,
			0,
			0,
			uintptr(WINEVENT_OUTOFCONTEXT|WINEVENT_SKIPOWNPROCESS),
		)
		if hWinEventHook == 0 {
			fmt.Println("[ProcessWatch] Failed to set WinEvent hook")
			return
		}
		defer procUnhookWinEvent.Call(hWinEventHook)

		fmt.Println("[ProcessWatch] Hook set successfully")
		enumWindows(func(hwnd uintptr) bool {
			handleProcessByHwnd(hwnd, EVENT_OBJECT_SHOW)
			return true
		})
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
	monitorMu.Lock()
	defer monitorMu.Unlock()

	if monitorDone != nil {
		fmt.Println("[PopupMonitor] Already running or stopping")
		return
	}

	done := make(chan struct{})
	ready := make(chan bool)

	monitorDone = done
	monitorStopping = false

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var h uintptr

		defer func() {
			if h != 0 {
				ret, _, err := procUnhookWinEvent.Call(h)
				if ret == 0 {
					fmt.Printf("[PopupMonitor] Unhook failed: %v\n", err)
				}
			}

			monitorMu.Lock()
			hookHandle = 0
			monitorThreadID = 0
			monitorDone = nil
			monitorStopping = false
			close(done)
			monitorMu.Unlock()

			fmt.Println("[PopupMonitor] Stopped")
		}()

		threadID, _, _ := procGetCurrentThreadID.Call()

		var msg MSG
		procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0, 0,
		)

		var err error
		h, _, err = procSetWinEventHook.Call(
			uintptr(EVENT_OBJECT_CREATE),
			uintptr(EVENT_OBJECT_SHOW),
			0,
			popup_winEventProcCallback,
			0,
			0,
			uintptr(WINEVENT_OUTOFCONTEXT|WINEVENT_SKIPOWNPROCESS),
		)
		if h == 0 {
			fmt.Printf("[PopupMonitor] Set hook failed: %v\n", err)
			ready <- false
			return
		}

		hookHandle = h
		monitorThreadID = threadID
		ready <- true

		for {
			ret, _, err := procGetMessageW.Call(
				uintptr(unsafe.Pointer(&msg)), 0, 0, 0,
			)

			switch int32(ret) {
			case -1:
				fmt.Printf("[PopupMonitor] GetMessage failed: %v\n", err)
				return
			case 0:
				return
			}

			procDispatchMessageW.Call(
				uintptr(unsafe.Pointer(&msg)),
			)
		}
	}()

	if <-ready {
		fmt.Println("[PopupMonitor] Started")
	}
}

func stopMonitoringPopups() {
	monitorMu.Lock()

	if monitorDone == nil {
		monitorMu.Unlock()
		return
	}

	done := monitorDone

	if !monitorStopping {
		if monitorThreadID != 0 {
			ret, _, err := procPostThreadMessageW.Call(
				monitorThreadID,
				uintptr(WM_QUIT),
				0,
				0,
			)
			if ret == 0 {
				monitorMu.Unlock()
				fmt.Printf("[PopupMonitor] Send WM_QUIT failed: %v\n", err)
				return
			}
		}

		monitorStopping = true
	}

	monitorMu.Unlock()

	<-done
}
