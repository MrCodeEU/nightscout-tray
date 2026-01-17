package main

import (
	"embed"
	"log"
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
	tray.SetTooltip("") // Empty tooltip - we use the popup window instead

	// Create tray menu
	trayMenu := application.NewMenu()
	trayMenu.Add("Show Dashboard").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(_ *application.Context) {
		wailsApp.Quit()
	})
	tray.SetMenu(trayMenu)

	// Show popup window on hover using OnMouseEnter/OnMouseLeave
	tray.OnMouseEnter(func() {
		// Position window near the tray icon and show it
		_ = tray.PositionWindow(trayWindow, 5)
		trayWindow.Show()
	})

	tray.OnMouseLeave(func() {
		// Hide the popup when mouse leaves the tray icon
		// Small delay handled by WindowDebounce
		trayWindow.Hide()
	})

	// Add debounce to prevent flickering
	tray.WindowDebounce(200 * time.Millisecond)

	// Left click opens main dashboard
	tray.OnClick(func() {
		if !mainWindow.IsVisible() {
			mainWindow.Show()
			mainWindow.Focus()
		}
	})

	// Pass tray reference to service for dynamic updates
	nsService.SetTray(tray)
	nsService.SetApp(wailsApp)

	// Start the app
	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
