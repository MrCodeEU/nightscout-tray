# Nightscout Tray

**Nightscout Tray** is a cross-platform system tray application designed to monitor glucose levels from a Nightscout instance. It sits quietly in your system tray, providing real-time glucose values, trend arrows, and alerts.

## Project Structure

This project is a hybrid desktop application built with **Go** and **Wails**, utilizing a web frontend.

*   **Backend (Go):** Handles the system tray integration, Nightscout API communication, notifications, and application state.
    *   `main.go`: Application entry point.
    *   `internal/app`: Core application logic and methods exposed to the frontend (Wails bindings).
    *   `internal/nightscout`: Client for the Nightscout API.
    *   `internal/tray`: System tray icon and menu management.
    *   `internal/notifications`: Notification logic.
    *   `internal/autostart`: OS-specific auto-start functionality.
*   **Frontend (Web):** Provides the configuration UI and detailed charts.
    *   `frontend/`: Contains the Vite-based web application.
*   **Configuration:**
    *   `wails.json`: Wails project configuration.
    *   `Makefile`: Build and maintenance automation.

## Building and Running

The project uses a `Makefile` to simplify common tasks. Ensure you have **Go 1.22+**, **Node.js 18+**, and the **Wails CLI** installed.

### Key Commands

*   **Install Dependencies:**
    ```bash
    make install-deps
    ```
    Downloads Go modules and installs npm packages in the `frontend` directory.

*   **Development Mode (Live Reload):**
    ```bash
    make dev
    # OR directly via Wails
    wails dev
    ```
    Compiles the app and runs it. Changes to Go or frontend code usually trigger a rebuild/reload.

*   **Production Build:**
    ```bash
    make build
    ```
    Builds the application for the current platform. The binary will be placed in `build/bin`.

*   **Build Frontend Only:**
    ```bash
    make frontend
    ```

*   **Running Tests:**
    ```bash
    make test
    ```
    Runs Go unit tests with race detection.

*   **Linting:**
    ```bash
    make lint
    ```
    Runs `golangci-lint` on the Go codebase.

## Development Conventions

*   **Wails Architecture:** The application logic resides in `internal/app/app.go`. Methods on the `App` struct are bound to the frontend, allowing JavaScript to call Go functions directly.
*   **Frontend:** The frontend is a standard web application (likely React, Vue, or vanilla JS/Vite - check `frontend/src` for specifics) bundled into the Go binary.
*   **Settings:** User settings are managed via `internal/models/settings.go` and persisted locally.
*   **Cross-Platform:** The code is designed to run on Linux, Windows, and macOS. Platform-specific code (like autostart or tray implementation details) should be handled carefully.
*   **Testing:** New features should include unit tests (`*_test.go`).
