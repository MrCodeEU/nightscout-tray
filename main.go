// Package main is the entry point for the Nightscout Tray application
package main

// change brail to better unicode + compact on windows 1 less wide + DRY use existing code?
import (
	"embed"
	"io"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/mrcode/nightscout-tray/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

// filteredLogWriter filters out false positive systray errors on Windows
type filteredLogWriter struct {
	writer io.Writer
}

func (w *filteredLogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)

	// On Windows, filter out false positive systray errors that indicate success
	// The German message "Der Vorgang wurde erfolgreich beendet" means "The operation completed successfully"
	// This is a bug in the energye/systray library where it logs success as an error
	if runtime.GOOS == "windows" {
		if strings.Contains(msg, "systray error: unable to set icon") &&
			(strings.Contains(msg, "Der Vorgang wurde erfolgreich beendet") ||
				strings.Contains(msg, "The operation completed successfully") ||
				strings.Contains(msg, "L'opération a réussi")) { // French
			return len(p), nil // Swallow the false error
		}
	}

	return w.writer.Write(p)
}

func main() {
	// Set up filtered logging on Windows to suppress false systray errors
	if runtime.GOOS == "windows" {
		log.SetOutput(&filteredLogWriter{writer: os.Stderr})
	}

	// Create application instance
	application := app.New()

	// Create Wails application
	err := wails.Run(&options.App{
		Title:             "Nightscout Tray",
		Width:             900,
		Height:            700,
		MinWidth:          600,
		MinHeight:         500,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       true, // Start hidden, show from tray
		HideWindowOnClose: true, // Hide instead of close
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        application.Startup,
		OnShutdown:       application.Shutdown,
		OnBeforeClose:    application.BeforeClose,
		Bind: []interface{}{
			application,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: false,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
