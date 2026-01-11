package nightscout

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

func TestHashSecret(t *testing.T) {
	result := hashSecret("test")
	expected := "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3"

	if result != expected {
		t.Errorf("hashSecret(\"test\") = %s, want %s", result, expected)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("https://test.example.com", "secret", "token", true)

	if client.baseURL != "https://test.example.com" {
		t.Errorf("baseURL = %s, want https://test.example.com", client.baseURL)
	}
	if client.apiSecret != "secret" {
		t.Errorf("apiSecret = %s, want secret", client.apiSecret)
	}
	if client.apiToken != "token" {
		t.Errorf("apiToken = %s, want token", client.apiToken)
	}
	if !client.useToken {
		t.Error("useToken should be true")
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client := NewClient("https://test.example.com/", "", "", false)

	if client.baseURL != "https://test.example.com" {
		t.Errorf("baseURL = %s, should not have trailing slash", client.baseURL)
	}
}

func TestClient_GetCurrentEntry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entries/current" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		entry := models.GlucoseEntry{
			ID:        "test123",
			SGV:       120,
			Date:      time.Now().UnixMilli(),
			Direction: "Flat",
			Trend:     4,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entry)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	entry, err := client.GetCurrentEntry()

	if err != nil {
		t.Fatalf("GetCurrentEntry() error = %v", err)
	}
	if entry.SGV != 120 {
		t.Errorf("SGV = %d, want 120", entry.SGV)
	}
	if entry.Direction != "Flat" {
		t.Errorf("Direction = %s, want Flat", entry.Direction)
	}
}

func TestClient_GetCurrentEntry_Array(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entries := []models.GlucoseEntry{
			{
				ID:        "test123",
				SGV:       130,
				Date:      time.Now().UnixMilli(),
				Direction: "SingleUp",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	entry, err := client.GetCurrentEntry()

	if err != nil {
		t.Fatalf("GetCurrentEntry() error = %v", err)
	}
	if entry.SGV != 130 {
		t.Errorf("SGV = %d, want 130", entry.SGV)
	}
}

func TestClient_GetEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entries/sgv" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		entries := []models.GlucoseEntry{
			{SGV: 120, Date: time.Now().UnixMilli()},
			{SGV: 115, Date: time.Now().Add(-5 * time.Minute).UnixMilli()},
			{SGV: 118, Date: time.Now().Add(-10 * time.Minute).UnixMilli()},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	from := time.Now().Add(-1 * time.Hour)
	entries, err := client.GetEntries(from, time.Time{}, 0)

	if err != nil {
		t.Fatalf("GetEntries() error = %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Got %d entries, want 3", len(entries))
	}
}

func TestClient_GetStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		status := models.ServerStatus{
			Status:     "ok",
			Name:       "test-nightscout",
			Version:    "14.0.0",
			APIEnabled: true,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	status, err := client.GetStatus()

	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if status.Status != "ok" {
		t.Errorf("Status = %s, want ok", status.Status)
	}
	if status.Name != "test-nightscout" {
		t.Errorf("Name = %s, want test-nightscout", status.Name)
	}
}

func TestClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		status := models.ServerStatus{Status: "ok"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	err := client.TestConnection()

	if err != nil {
		t.Errorf("TestConnection() error = %v, want nil", err)
	}
}

func TestClient_AuthHeaders_Token(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer testtoken123" {
			t.Errorf("Authorization header = %s, want Bearer testtoken123", authHeader)
		}

		status := models.ServerStatus{Status: "ok"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "testtoken123", true)
	_, _ = client.GetStatus()
}

func TestClient_AuthHeaders_Secret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secretHeader := r.Header.Get("API-SECRET")
		expectedHash := hashSecret("mysecret")
		if secretHeader != expectedHash {
			t.Errorf("API-SECRET header = %s, want %s", secretHeader, expectedHash)
		}

		status := models.ServerStatus{Status: "ok"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	client := NewClient(server.URL, "mysecret", "", false)
	_, _ = client.GetStatus()
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "", false)
	_, err := client.GetStatus()

	if err == nil {
		t.Error("Expected error for 401 response")
	}
}
