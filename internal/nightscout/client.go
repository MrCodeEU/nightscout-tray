// Package nightscout provides a client for interacting with the Nightscout API
package nightscout

import (
	"crypto/sha1" //nolint:gosec // Required for Nightscout API secret hashing (legacy API requirement)
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mrcode/nightscout-tray/internal/models"
)

// Client handles communication with the Nightscout API
type Client struct {
	baseURL    string
	apiSecret  string
	apiToken   string
	useToken   bool
	httpClient *http.Client
}

// NewClient creates a new Nightscout client
func NewClient(baseURL, apiSecret, apiToken string, useToken bool) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiSecret: apiSecret,
		apiToken:  apiToken,
		useToken:  useToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// hashSecret generates SHA1 hash of the API secret
// Note: SHA1 is required for Nightscout API compatibility
func hashSecret(secret string) string {
	hasher := sha1.New() //nolint:gosec // Required for Nightscout API
	hasher.Write([]byte(secret))
	return hex.EncodeToString(hasher.Sum(nil))
}

// buildRequest creates an HTTP request with proper authentication
func (c *Client) buildRequest(method, endpoint string, params url.Values) (*http.Request, error) {
	fullURL := c.baseURL + endpoint
	if params != nil {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if c.useToken && c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	} else if c.apiSecret != "" {
		req.Header.Set("API-SECRET", hashSecret(c.apiSecret))
	}

	return req, nil
}

// doRequest executes an HTTP request and returns the response body
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetStatus retrieves the Nightscout server status
func (c *Client) GetStatus() (*models.ServerStatus, error) {
	req, err := c.buildRequest("GET", "/api/v1/status", nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var status models.ServerStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("parsing status: %w", err)
	}

	return &status, nil
}

// GetCurrentEntry retrieves the most recent glucose entry
func (c *Client) GetCurrentEntry() (*models.GlucoseEntry, error) {
	params := url.Values{}
	params.Set("count", "1")

	req, err := c.buildRequest("GET", "/api/v1/entries/current", params)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	// Current endpoint returns a single object or array
	var entry models.GlucoseEntry
	if err := json.Unmarshal(body, &entry); err != nil {
		// Try as array
		var entries []models.GlucoseEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, fmt.Errorf("parsing entry: %w", err)
		}
		if len(entries) > 0 {
			return &entries[0], nil
		}
		return nil, fmt.Errorf("no entries returned")
	}

	return &entry, nil
}

// GetEntries retrieves glucose entries for a time range
func (c *Client) GetEntries(from, to time.Time, count int) ([]models.GlucoseEntry, error) {
	params := url.Values{}

	if !from.IsZero() {
		params.Set("find[date][$gte]", fmt.Sprintf("%d", from.UnixMilli()))
	}
	if !to.IsZero() {
		params.Set("find[date][$lte]", fmt.Sprintf("%d", to.UnixMilli()))
	}
	if count > 0 {
		params.Set("count", fmt.Sprintf("%d", count))
	}

	req, err := c.buildRequest("GET", "/api/v1/entries/sgv", params)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var entries []models.GlucoseEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing entries: %w", err)
	}

	return entries, nil
}

// GetEntriesHours retrieves glucose entries for the last N hours
func (c *Client) GetEntriesHours(hours int) ([]models.GlucoseEntry, error) {
	from := time.Now().Add(-time.Duration(hours) * time.Hour)
	return c.GetEntries(from, time.Time{}, 0)
}

// GetEntriesDays retrieves glucose entries for the last N days
func (c *Client) GetEntriesDays(days int) ([]models.GlucoseEntry, error) {
	from := time.Now().AddDate(0, 0, -days)
	return c.GetEntries(from, time.Time{}, 0)
}

// TestConnection tests if the connection to Nightscout works
func (c *Client) TestConnection() error {
	_, err := c.GetStatus()
	return err
}

// GetRecentEntries retrieves the most recent N entries
func (c *Client) GetRecentEntries(count int) ([]models.GlucoseEntry, error) {
	params := url.Values{}
	params.Set("count", fmt.Sprintf("%d", count))

	req, err := c.buildRequest("GET", "/api/v1/entries/sgv", params)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var entries []models.GlucoseEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing entries: %w", err)
	}

	return entries, nil
}
