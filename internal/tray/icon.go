// Package tray handles the system tray icon and menu
package tray

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"runtime"
	"sync"

	"github.com/energye/systray"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/mrcode/nightscout-tray/internal/models"
	"golang.org/x/image/font/gofont/goregular"
)

// Icon represents the tray icon manager
type Icon struct {
	mu         sync.Mutex
	settings   *models.Settings
	onShow     func()
	onQuit     func()
	menuShow   *systray.MenuItem
	menuQuit   *systray.MenuItem
	lastStatus *models.GlucoseStatus
	history    []float64 // Last 24 values for sparkline (2 hours)
}

// NewIcon creates a new tray icon manager
func NewIcon(settings *models.Settings, onShow, onQuit func()) *Icon {
	return &Icon{
		settings: settings,
		onShow:   onShow,
		onQuit:   onQuit,
		history:  make([]float64, 0, 24),
	}
}

// Run starts the system tray - must be called from main goroutine
func (t *Icon) Run() {
	systray.Run(t.onReady, t.onExit)
}

// Quit exits the system tray
func (t *Icon) Quit() {
	systray.Quit()
}

// onReady is called when the tray is ready
func (t *Icon) onReady() {
	systray.SetIcon(t.generateIcon("---", ""))
	systray.SetTitle("Nightscout Tray")
	systray.SetTooltip("Nightscout Tray - Loading...")

	// Handle left click to open dashboard
	systray.SetOnClick(func(_ systray.IMenu) {
		if t.onShow != nil {
			t.onShow()
		}
	})

	// Create menu items
	t.menuShow = systray.AddMenuItem("Open Dashboard", "Open the main window")
	systray.AddSeparator()
	t.menuQuit = systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks using callback functions
	t.menuShow.Click(func() {
		if t.onShow != nil {
			t.onShow()
		}
	})

	t.menuQuit.Click(func() {
		if t.onQuit != nil {
			t.onQuit()
		}
	})
}

// onExit is called when the tray is being closed
func (t *Icon) onExit() {
	// Cleanup if needed
}

// UpdateStatus updates the tray icon with the current glucose status
func (t *Icon) UpdateStatus(status *models.GlucoseStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastStatus = status

	// Update history
	val := float64(status.Value)
	if t.settings.Unit == "mmol/L" {
		val = status.ValueMmol
	}
	t.history = append(t.history, val)
	// Keep last 24 entries (prevent tooltip wrapping)
	if len(t.history) > 24 {
		t.history = t.history[1:]
	}

	var valueStr string
	if t.settings.Unit == "mmol/L" {
		valueStr = fmt.Sprintf("%.1f", status.ValueMmol)
	} else {
		valueStr = fmt.Sprintf("%d", status.Value)
	}

	// Update tray title (shows next to icon on some platforms)
	displayText := fmt.Sprintf("%s %s", valueStr, status.Trend)
	systray.SetTitle(displayText)

	// Update tooltip with rich info
	sparkline := t.generateMultiLineSparkline()

	// Note: Native tooltips generally do not support color or rich text.
	// We rely on ASCII/Unicode art for the chart.
	tooltip := fmt.Sprintf("%s %s %s\n%s\nStatus: %s\nUpdated: %s ago",
		valueStr, t.settings.Unit, status.Trend,
		sparkline,
		t.formatStatus(status.Status),
		t.formatDuration(status.StaleMinutes))

	if status.IsStale {
		tooltip += "\n⚠️ Data may be stale"
	}

	systray.SetTooltip(tooltip)

	// Update icon with current value
	iconBytes := t.generateIcon(valueStr, status.Direction)
	if iconBytes != nil {
		systray.SetIcon(iconBytes)
	}
}

// SetError sets an error state on the tray
func (t *Icon) SetError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	systray.SetTitle("⚠️")
	systray.SetTooltip(fmt.Sprintf("Error: %v", err))
	systray.SetIcon(t.generateIcon("ERR", ""))
}

// SetLoading sets a loading state on the tray
func (t *Icon) SetLoading() {
	t.mu.Lock()
	defer t.mu.Unlock()

	systray.SetTitle("...")
	systray.SetTooltip("Loading glucose data...")
}

// formatStatus returns a human-readable status string
func (t *Icon) formatStatus(status string) string {
	switch status {
	case "urgent_low":
		return "Urgent Low"
	case "urgent_high":
		return "Urgent High"
	case "low":
		return "Low"
	case "high":
		return "High"
	case "normal":
		return "In Range"
	default:
		return status
	}
}

// formatDuration formats minutes into a human-readable duration
func (t *Icon) formatDuration(minutes int) string {
	if minutes < 1 {
		return "just now"
	}
	if minutes == 1 {
		return "1 minute"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	hours := minutes / 60
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}

// generateMultiLineSparkline creates a multi-line ASCII chart

func (t *Icon) generateMultiLineSparkline() string {

	if len(t.history) < 2 {

		return ""

	}

	// Constants

	height := 10 // Balanced height

	minVal := t.history[0]

	maxVal := t.history[0]

	for _, v := range t.history {

		if v < minVal {

			minVal = v

		}

		if v > maxVal {

			maxVal = v

		}

	}

	// Dynamic scaling with buffer
	buffer := 10.0
	minVal = math.Max(0, minVal-buffer)
	maxVal += buffer
	rangeVal := maxVal - minVal

	// Braille blocks for better alignment and resolution (4 sub-blocks high)
	// Empty, 1/4, 1/2, 3/4, Full
	blocks := []rune{'⠀', '⣀', '⣤', '⣶', '⣿'}
	subBlocksPerLine := 4.0

	rows := make([][]rune, height)
	width := len(t.history)
	for i := 0; i < height; i++ {
		rows[i] = make([]rune, width)
		for j := 0; j < width; j++ {
			rows[i][j] = '⠀' // Empty Braille space
		}
	}

	for x, val := range t.history {
		normalized := (val - minVal) / rangeVal
		// Total "height" in sub-blocks
		totalSubBlocks := normalized * float64(height) * subBlocksPerLine

		// Fill lines from bottom up
		for y := 0; y < height; y++ {
			// Line index from bottom (0 is bottom line)
			lineIdx := height - 1 - y
			// Range covered by this line
			lineStart := float64(y) * subBlocksPerLine
			lineEnd := float64(y+1) * subBlocksPerLine

			if totalSubBlocks >= lineEnd {
				// Full block
				rows[lineIdx][x] = '⣿'
			} else if totalSubBlocks > lineStart {
				// Partial block
				// Use rounding for better accuracy
				remainder := int(math.Round(totalSubBlocks - lineStart))
				if remainder < 0 {
					remainder = 0
				}
				if remainder >= len(blocks) {
					remainder = len(blocks) - 1
				}
				rows[lineIdx][x] = blocks[remainder]
			}
		}
	}

	var result bytes.Buffer

	result.WriteString("\n") // Start on new line

	// Top Label (Max)

	result.WriteString(fmt.Sprintf("Max: %.0f\n", maxVal))

	// Chart lines

	for i := 0; i < height; i++ {

		result.WriteString(string(rows[i]))

		result.WriteString("\n")

	}

	// Bottom Label (Min)

	result.WriteString(fmt.Sprintf("Min: %.0f", minVal))

	return result.String()

}

// generateIcon generates an icon with text using gg
func (t *Icon) generateIcon(text string, direction string) []byte {
	// Size constants
	const (
		width  = 64 // Higher resolution for better scaling
		height = 64
		radius = 16 // More rounded
	)

	dc := gg.NewContext(width, height)

	// Transparent background
	dc.SetRGBA(0, 0, 0, 0)
	dc.Clear()

	// Get status color
	bgHex := t.getStatusColor()
	r, g, b := parseHexColor(bgHex)

	// Draw rounded rectangle background
	dc.SetRGB255(int(r), int(g), int(b))
	dc.DrawRoundedRectangle(0, 0, float64(width), float64(height), float64(radius))
	dc.Fill()

	// Text color (black or white depending on brightness)
	brightness := (int(r)*299 + int(g)*587 + int(b)*114) / 1000
	if brightness > 128 {
		dc.SetColor(color.Black)
	} else {
		dc.SetColor(color.White)
	}

	// Draw value (Text)
	// Moved up significantly: y was height/2-8, now height/2-10
	if err := t.loadFont(dc, 34); err == nil {
		dc.DrawStringAnchored(text, width/2, height/2-12, 0.5, 0.5)
	}

	// Draw arrow (Shape)
	// Moved up: y was height-12, now height-14
	if direction != "" {
		t.drawArrow(dc, width/2, height-16, 24, direction)
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return nil
	}

	return buf.Bytes()
}

// loadFont helper to load font safely
func (t *Icon) loadFont(dc *gg.Context, size float64) error {
	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return err
	}
	face := truetype.NewFace(font, &truetype.Options{Size: size})
	dc.SetFontFace(face)
	return nil
}

// drawArrow draws a vector arrow based on direction
func (t *Icon) drawArrow(dc *gg.Context, x, y, size float64, direction string) {
	dc.Push()
	defer dc.Pop()

	// Translate to center of arrow
	dc.Translate(x, y)

	// Rotate based on direction
	var angle float64
	switch direction {
	case "DoubleUp", "SingleUp":
		angle = 0
	case "FortyFiveUp":
		angle = 45
	case "Flat":
		angle = 90
	case "FortyFiveDown":
		angle = 135
	case "DoubleDown", "SingleDown":
		angle = 180
	default:
		return // No arrow
	}

	dc.Rotate(gg.Radians(angle))

	// Draw arrow shape
	// Simple triangle/pointer
	//       ^
	//      / \
	//     / | \
	//    /  |  \
	//       |

	halfSize := size / 2

	if direction == "DoubleUp" || direction == "DoubleDown" {
		// Draw double arrow
		t.drawSingleArrow(dc, 0, -halfSize/2, size*0.8)
		t.drawSingleArrow(dc, 0, halfSize/2, size*0.8)
	} else {
		t.drawSingleArrow(dc, 0, 0, size)
	}
}

func (t *Icon) drawSingleArrow(dc *gg.Context, ox, oy, s float64) {
	// Standard arrow shape centered at ox, oy
	w := s * 0.5 // Width

	dc.NewSubPath() // Tip
	dc.MoveTo(ox, oy-s/2)
	// Right corner
	dc.LineTo(ox+w/2, oy)
	// Shaft right
	dc.LineTo(ox+w/6, oy)
	// Shaft bottom right
	dc.LineTo(ox+w/6, oy+s/2)
	// Shaft bottom left
	dc.LineTo(ox-w/6, oy+s/2)
	// Shaft left
	dc.LineTo(ox-w/6, oy)
	// Left corner
	dc.LineTo(ox-w/2, oy)
	dc.ClosePath()
	dc.Fill()
}

// getStatusColor returns the color based on the last known status
func (t *Icon) getStatusColor() string {
	if t.lastStatus == nil {
		return "#808080" // Gray for unknown
	}

	switch t.lastStatus.Status {
	case "urgent_low", "urgent_high":
		return "#ef4444" // Red
	case "low":
		return "#f97316" // Orange
	case "high":
		return "#facc15" // Yellow
	default:
		return "#4ade80" // Green
	}
}

// parseHexColor parses a hex color string to RGB values
func parseHexColor(hex string) (r, g, b byte) {
	if len(hex) == 7 && hex[0] == '#' {
		_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	}
	return
}

// UpdateSettings updates the settings reference
func (t *Icon) UpdateSettings(settings *models.Settings) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.settings = settings

	// Clear history on settings change to avoid unit mixup
	t.history = make([]float64, 0, 24)

	// Re-render with new settings
	if t.lastStatus != nil {
		go t.UpdateStatus(t.lastStatus)
	}
}

// IsTraySupported returns true if system tray is supported on this platform
func IsTraySupported() bool {
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		return true
	default:
		return false
	}
}
