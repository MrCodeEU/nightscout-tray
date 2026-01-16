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
// Uses pagination to handle Nightscout's API limits
func (c *Client) GetEntries(from, to time.Time, count int) ([]models.GlucoseEntry, error) {
	// Nightscout API has a max limit per request (typically 10,000)
	// We need to paginate to get all data for large date ranges
	const maxPerRequest = 10000

	var allEntries []models.GlucoseEntry
	currentTo := to
	if currentTo.IsZero() {
		currentTo = time.Now()
	}

	pageNum := 0
	for {
		pageNum++
		params := url.Values{}
		if !from.IsZero() {
			params.Set("find[date][$gte]", fmt.Sprintf("%d", from.UnixMilli()))
		}
		params.Set("find[date][$lte]", fmt.Sprintf("%d", currentTo.UnixMilli()))
		params.Set("count", fmt.Sprintf("%d", maxPerRequest))

		fmt.Printf("GetEntries page %d: from=%s to=%s\n", pageNum, from.Format("2006-01-02"), currentTo.Format("2006-01-02 15:04"))
		
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
		
		fmt.Printf("  Page %d: received %d entries\n", pageNum, len(entries))

		if len(entries) == 0 {
			break
		}

		allEntries = append(allEntries, entries...)
		fmt.Printf("  Total entries so far: %d\n", len(allEntries))

		// Check if we got fewer entries than requested - we've reached the end
		if len(entries) < maxPerRequest {
			fmt.Println("  Less than maxPerRequest, done paginating")
			break
		}

		// NOTE: We do NOT stop based on count when we have a date range (from is set)
		// The count parameter is only used for non-date-range queries
		// For date-range queries, we paginate until we cover the full range
		if count > 0 && from.IsZero() && len(allEntries) >= count {
			allEntries = allEntries[:count]
			break
		}

		// Find the oldest entry's timestamp for the next page
		// Entries are typically returned newest first
		oldestTime := time.Now()
		for _, e := range entries {
			entryTime := e.Time() // Call the Time() method
			if entryTime.Before(oldestTime) {
				oldestTime = entryTime
			}
		}

		// Set the next page to end just before the oldest entry
		// Subtract 1 millisecond to avoid duplicate entries
		currentTo = oldestTime.Add(-time.Millisecond)

		// Check if we've gone past the requested start time
		if !from.IsZero() && currentTo.Before(from) {
			break
		}
	}
	
	// Log final data range
	if len(allEntries) > 0 {
		// Find oldest and newest
		oldest := time.Now()
		newest := time.Time{}
		for _, e := range allEntries {
			t := e.Time()
			if t.Before(oldest) {
				oldest = t
			}
			if t.After(newest) {
				newest = t
			}
		}
		days := newest.Sub(oldest).Hours() / 24
		fmt.Printf("GetEntries complete: %d entries spanning %.1f days (%s to %s)\n", 
			len(allEntries), days, oldest.Format("2006-01-02"), newest.Format("2006-01-02"))
	}

	return allEntries, nil
}

// GetEntriesHours retrieves glucose entries for the last N hours
// Note: We request a larger count to ensure all entries are returned
func (c *Client) GetEntriesHours(hours int) ([]models.GlucoseEntry, error) {
	from := time.Now().Add(-time.Duration(hours) * time.Hour)
	// ~12 readings per hour with 5-minute intervals + buffer
	count := hours * 15
	return c.GetEntries(from, time.Time{}, count)
}

// GetEntriesDays retrieves glucose entries for the last N days
// Note: We request a large count because Nightscout has a default limit
// (~288 readings per day with 5-minute intervals)
func (c *Client) GetEntriesDays(days int) ([]models.GlucoseEntry, error) {
	from := time.Now().AddDate(0, 0, -days)
	// Calculate expected count: 288 readings/day * days + buffer
	count := days * 300
	return c.GetEntries(from, time.Time{}, count)
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

// GetTreatments retrieves treatment entries for a time range
// Uses pagination to handle Nightscout's API limits
func (c *Client) GetTreatments(from, to time.Time, count int) ([]models.Treatment, error) {
	// Nightscout API has a max limit per request (typically 10,000)
	const maxPerRequest = 10000

	var allTreatments []models.Treatment
	currentTo := to
	if currentTo.IsZero() {
		currentTo = time.Now()
	}

	for {
		params := url.Values{}
		if !from.IsZero() {
			params.Set("find[created_at][$gte]", from.Format(time.RFC3339))
		}
		params.Set("find[created_at][$lte]", currentTo.Format(time.RFC3339))
		params.Set("count", fmt.Sprintf("%d", maxPerRequest))

		req, err := c.buildRequest("GET", "/api/v1/treatments", params)
		if err != nil {
			return nil, err
		}

		body, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}

		var treatments []models.Treatment
		if err := json.Unmarshal(body, &treatments); err != nil {
			return nil, fmt.Errorf("parsing treatments: %w", err)
		}

		if len(treatments) == 0 {
			break
		}

		allTreatments = append(allTreatments, treatments...)

		// Check if we got fewer treatments than requested - we've reached the end
		if len(treatments) < maxPerRequest {
			break
		}

		// NOTE: We do NOT stop based on count when we have a date range (from is set)
		// For date-range queries, we paginate until we cover the full range
		if count > 0 && from.IsZero() && len(allTreatments) >= count {
			allTreatments = allTreatments[:count]
			break
		}

		// Find the oldest treatment's timestamp for the next page
		oldestTime := time.Now()
		for _, t := range treatments {
			treatmentTime := t.Time() // Call the Time() method
			if treatmentTime.Before(oldestTime) {
				oldestTime = treatmentTime
			}
		}

		// Set the next page to end just before the oldest treatment
		currentTo = oldestTime.Add(-time.Second)

		// Check if we've gone past the requested start time
		if !from.IsZero() && currentTo.Before(from) {
			break
		}
	}

	return allTreatments, nil
}

// GetTreatmentsDays retrieves treatments for the last N days
func (c *Client) GetTreatmentsDays(days int) ([]models.Treatment, error) {
	from := time.Now().AddDate(0, 0, -days)
	// Request a large count to ensure we get all treatments
	count := days * 50 // Estimate ~50 treatments per day max
	return c.GetTreatments(from, time.Time{}, count)
}

// GetTreatmentsHours retrieves treatments for the last N hours
func (c *Client) GetTreatmentsHours(hours int) ([]models.Treatment, error) {
	from := time.Now().Add(-time.Duration(hours) * time.Hour)
	// Estimate ~5 treatments per hour max
	count := hours * 10
	return c.GetTreatments(from, time.Time{}, count)
}

// GetRecentTreatments retrieves the most recent N treatments
func (c *Client) GetRecentTreatments(count int) ([]models.Treatment, error) {
	params := url.Values{}
	params.Set("count", fmt.Sprintf("%d", count))

	req, err := c.buildRequest("GET", "/api/v1/treatments", params)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var treatments []models.Treatment
	if err := json.Unmarshal(body, &treatments); err != nil {
		return nil, fmt.Errorf("parsing treatments: %w", err)
	}

	return treatments, nil
}

// GetInsulinTreatments retrieves only insulin-related treatments
func (c *Client) GetInsulinTreatments(from, to time.Time) ([]models.Treatment, error) {
	treatments, err := c.GetTreatments(from, to, 0)
	if err != nil {
		return nil, err
	}

	var insulinTreatments []models.Treatment
	for _, t := range treatments {
		if t.HasInsulin() {
			insulinTreatments = append(insulinTreatments, t)
		}
	}

	return insulinTreatments, nil
}

// GetCarbTreatments retrieves only carb-related treatments
func (c *Client) GetCarbTreatments(from, to time.Time) ([]models.Treatment, error) {
	treatments, err := c.GetTreatments(from, to, 0)
	if err != nil {
		return nil, err
	}

	var carbTreatments []models.Treatment
	for _, t := range treatments {
		if t.HasCarbs() {
			carbTreatments = append(carbTreatments, t)
		}
	}

	return carbTreatments, nil
}
