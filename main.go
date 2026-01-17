package main

import (
	"embed"
	"log"
	"runtime"
	"time"

	"github.com/mrcode/nightscout-tray/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	nsService := app.NewNightscoutService()

	wailsApp := application.New(application.Options{
		Name:        "Nightscout Tray",
		Description: "Nightscout CGM Monitoring",
		Services: []application.Service{
			application.NewService(nsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
			ActivationPolicy:                                application.ActivationPolicyAccessory,
		},
	})

	// Main Dashboard Window - starts hidden
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   "main",
		Title:  "Nightscout Dashboard",
		Width:  900,
		Height: 700,
		Hidden: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	// System Tray Popup Window - attached to tray, frameless
	trayWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "tray-popup",
		Title:            "",
		Width:            320,
		Height:           280,
		Frameless:        true,
		AlwaysOnTop:      true,
		Hidden:           true,
		DisableResize:    true,
		BackgroundColour: application.NewRGB(30, 41, 59),
		URL:              "/#/tray",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
		},
	})

	// Create System Tray
	tray := wailsApp.SystemTray.New()

	// Set initial icon from embedded PNG
	tray.SetIcon(appIcon)
	tray.SetLabel("...")
	
	// On Linux, show tooltip since hover popup doesn't work
	// On Windows, we use the popup window on hover instead
	if runtime.GOOS == "windows" {
		tray.SetTooltip("")
	} else {
		tray.SetTooltip("Nightscout Tray - Loading...")
	}

	// Create tray menu
	trayMenu := application.NewMenu()
	trayMenu.Add("Open Dashboard").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		wailsApp.Quit()
	})
	tray.SetMenu(trayMenu)

	// Platform-specific tray behavior
	if runtime.GOOS == "windows" {
		// Windows: Hover shows popup, left click opens dashboard
		tray.OnMouseEnter(func() {
			_ = tray.PositionWindow(trayWindow, 5)
			trayWindow.Show()
		})

		tray.OnMouseLeave(func() {
			trayWindow.Hide()
		})

		tray.WindowDebounce(200 * time.Millisecond)

		tray.OnClick(func() {
			if !mainWindow.IsVisible() {
				mainWindow.Show()
				mainWindow.Focus()
			}
		})
	} else {
		// Linux/macOS: Left click toggles popup, right-click menu for dashboard
		// Note: PositionWindow may not work on Linux (StatusNotifierItem doesn't provide position)
		// so we also manually position the window near the top-right corner where panels usually are
		var popupVisible bool
		tray.OnClick(func() {
			if popupVisible {
				trayWindow.Hide()
				popupVisible = false
			} else {
				// Try PositionWindow first (works on some systems)
				err := tray.PositionWindow(trayWindow, 5)
				if err != nil {
					// Fallback: position in top-right corner
					// Most Linux panels are at the top, and system tray is on the right
					// Use reasonable defaults for common screen resolutions
					// Window is 320x280, position it near top-right with some margin
					trayWindow.SetPosition(1550, 35)
				}
				trayWindow.Show()
				popupVisible = true
			}
		})
	}

	// Pass tray reference to service for dynamic updates
	nsService.SetTray(tray)
	nsService.SetApp(wailsApp)

	// Start the app
	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
