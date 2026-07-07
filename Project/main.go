package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var appIconBytes []byte

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	createMutex        = kernel32.NewProc("CreateMutexW")
	getLastError       = kernel32.NewProc("GetLastError")
	registerHotKey     = user32.NewProc("RegisterHotKey")
	unregisterHotKey   = user32.NewProc("UnregisterHotKey")
	getMessage         = user32.NewProc("GetMessageW")
	postThreadMessage  = user32.NewProc("PostThreadMessageW")
	getCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	findWindowW        = user32.NewProc("FindWindowW")
	getWindowLongW     = user32.NewProc("GetWindowLongW")
	setWindowLongW     = user32.NewProc("SetWindowLongW")
	showWindow         = user32.NewProc("ShowWindow")
)

const (
	GWL_EXSTYLE          = 0xFFFFFFEC
	WS_EX_TOOLWINDOW     = 0x00000080
	WS_EX_APPWINDOW      = 0x00040000
	SW_HIDE              = 0
	SW_SHOW              = 5
	ERROR_ALREADY_EXISTS = 183
	WM_HOTKEY            = 0x0312
	WM_USER              = 0x0400
	WM_UPDATE_HOTKEY     = WM_USER + 1
)

type MSG struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

var currentHotkeyThreadId uint32
var wailsCtx context.Context

func checkSingleInstance() (uintptr, error) {
	namePtr, err := syscall.UTF16PtrFromString("QuickAddSingleInstanceMutex")
	if err != nil {
		return 0, err
	}
	hMutex, _, errMutex := createMutex.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if errMutex != nil && errMutex.(syscall.Errno) == ERROR_ALREADY_EXISTS {
		return 0, fmt.Errorf("instance already running")
	}
	return hMutex, nil
}

func getIconBytes() []byte {
	// Try "../IconApp.png" (development mode inside Project/)
	if data, err := os.ReadFile("../IconApp.png"); err == nil {
		return data
	}
	// Try "IconApp.png" (production mode in root)
	if data, err := os.ReadFile("IconApp.png"); err == nil {
		return data
	}
	// Fallback to executable folder
	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		if data, err := os.ReadFile(filepath.Join(dir, "IconApp.png")); err == nil {
			return data
		}
	}
	return nil
}

func setupStealthMode(title string) {
	go func() {
		var hwnd uintptr
		titlePtr, _ := syscall.UTF16PtrFromString(title)

		// Wait up to 5 seconds for Wails to construct the window
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			hwnd, _, _ = findWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd != 0 {
				break
			}
		}

		if hwnd != 0 {
			// Modify Window Style to Hide Taskbar Icon (WS_EX_TOOLWINDOW)
			style, _, _ := getWindowLongW.Call(hwnd, uintptr(GWL_EXSTYLE))
			newStyle := (style &^ WS_EX_APPWINDOW) | WS_EX_TOOLWINDOW
			setWindowLongW.Call(hwnd, uintptr(GWL_EXSTYLE), newStyle)
		}
	}()
}

func parseHotkey(hotkeyStr string) (uint32, uint32) {
	var mods uint32
	var vk uint32

	parts := strings.Split(hotkeyStr, "+")
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		switch part {
		case "ctrl", "control":
			mods |= 0x0002 // MOD_CONTROL
		case "shift":
			mods |= 0x0004 // MOD_SHIFT
		case "alt":
			mods |= 0x0001 // MOD_ALT
		case "win", "super":
			mods |= 0x0008 // MOD_WIN
		default:
			if len(part) == 1 {
				vk = uint32(strings.ToUpper(part)[0])
			} else {
				switch part {
				case "space":
					vk = 0x20 // VK_SPACE
				case "enter":
					vk = 0x0D // VK_RETURN
				case "tab":
					vk = 0x09 // VK_TAB
				case "escape", "esc":
					vk = 0x1B // VK_ESCAPE
				}
				if strings.HasPrefix(part, "f") && len(part) > 1 {
					var fNum int
					fmt.Sscanf(part, "f%d", &fNum)
					if fNum >= 1 && fNum <= 12 {
						vk = uint32(0x70 + fNum - 1)
					}
				}
			}
		}
	}
	return mods, vk
}

func registerGlobalHotkey(hotkeyStr string, app *App) {
	mods, vk := parseHotkey(hotkeyStr)
	if vk == 0 {
		return
	}

	if currentHotkeyThreadId != 0 {
		postThreadMessage.Call(uintptr(currentHotkeyThreadId), WM_UPDATE_HOTKEY, 0, 0)
		time.Sleep(50 * time.Millisecond)
	}

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		tid, _, _ := getCurrentThreadId.Call()
		currentHotkeyThreadId = uint32(tid)

		ret, _, _ := registerHotKey.Call(0, 1, uintptr(mods), uintptr(vk))
		if ret == 0 {
			return
		}
		defer unregisterHotKey.Call(0, 1)

		var msg MSG
		for {
			res, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if res == 0 || res == ^uintptr(0) {
				break
			}
			if msg.Message == WM_HOTKEY {
				if msg.WParam == 1 {
					app.TriggerCapture()
				}
			} else if msg.Message == WM_UPDATE_HOTKEY {
				break
			}
		}
	}()
}

func setupSystray(app *App) {
	go systray.Run(func() {
		if len(appIconBytes) > 0 {
			systray.SetIcon(appIconBytes)
		}
		systray.SetTooltip("Quick-Add AI Helper")

		mEdit := systray.AddMenuItem("Edit", "Mở cài đặt")
		mExit := systray.AddMenuItem("Exit", "Thoát ứng dụng")

		go func() {
			for {
				select {
				case <-mEdit.ClickedCh:
					if wailsCtx != nil {
						wailsRuntime.EventsEmit(wailsCtx, "show-tab2", nil)
						wailsRuntime.WindowSetSize(wailsCtx, 800, 550)
						wailsRuntime.WindowCenter(wailsCtx)
						wailsRuntime.WindowShow(wailsCtx)
					}
				case <-mExit.ClickedCh:
					systray.Quit()
					if wailsCtx != nil {
						wailsRuntime.Quit(wailsCtx)
					}
					os.Exit(0)
				}
			}
		}()
	}, func() {
		// Exit cleanup if needed
	})
}

func main() {
	// 1. Single Instance Check
	hMutex, err := checkSingleInstance()
	if err != nil {
		fmt.Printf("Ứng dụng đang được chạy ngầm. Không khởi tạo bản mới.\n")
		os.Exit(0)
	}
	defer func() {
		// Keep reference to mutex handle to keep it alive for process duration
		if hMutex != 0 {
			_ = hMutex
		}
	}()

	// 2. Create app instance
	app := NewApp()

	// 3. Setup System Tray
	setupSystray(app)

	// 4. Setup Windows Taskbar Hide
	windowTitle := "QuickAddMainWindow"
	setupStealthMode(windowTitle)

	// 5. Run Wails Application
	err = wails.Run(&options.App{
		Title:             windowTitle,
		Width:             400,
		Height:            550,
		Frameless:         true, // Always frameless
		StartHidden:       true, // Runs silently in system tray
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0}, // transparent support
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			wailsCtx = ctx
			app.startup(ctx)
			
			// Register Global Hotkey
			registerGlobalHotkey(app.config.Hotkey, app)

			// Setup event listener for hotkey configuration updates
			wailsRuntime.EventsOn(ctx, "hotkey-changed", func(optionalData ...interface{}) {
				if len(optionalData) > 0 {
					if hotkeyStr, ok := optionalData[0].(string); ok {
						registerGlobalHotkey(hotkeyStr, app)
					}
				}
			})
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                  true,
			DisableFramelessWindowDecorations: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
