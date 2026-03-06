package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is an HTTP client for the Gas City API server.
// It wraps mutation endpoints so CLI commands can route writes
// through the API when a controller is running.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client targeting the given base URL
// (e.g., "http://127.0.0.1:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SuspendCity suspends the city via PATCH /v0/city.
func (c *Client) SuspendCity() error {
	return c.patchCity(true)
}

// ResumeCity resumes the city via PATCH /v0/city.
func (c *Client) ResumeCity() error {
	return c.patchCity(false)
}

func (c *Client) patchCity(suspend bool) error {
	body := map[string]any{"suspended": suspend}
	return c.doMutation("PATCH", "/v0/city", body)
}

// SuspendAgent suspends an agent via POST /v0/agent/{name}/suspend.
func (c *Client) SuspendAgent(name string) error {
	return c.doMutation("POST", "/v0/agent/"+url.PathEscape(name)+"/suspend", nil)
}

// ResumeAgent resumes an agent via POST /v0/agent/{name}/resume.
func (c *Client) ResumeAgent(name string) error {
	return c.doMutation("POST", "/v0/agent/"+url.PathEscape(name)+"/resume", nil)
}

// SuspendRig suspends a rig via POST /v0/rig/{name}/suspend.
func (c *Client) SuspendRig(name string) error {
	return c.doMutation("POST", "/v0/rig/"+url.PathEscape(name)+"/suspend", nil)
}

// ResumeRig resumes a rig via POST /v0/rig/{name}/resume.
func (c *Client) ResumeRig(name string) error {
	return c.doMutation("POST", "/v0/rig/"+url.PathEscape(name)+"/resume", nil)
}

// doMutation sends a mutation request and checks for errors.
func (c *Client) doMutation(method, path string, body any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-GC-Request", "true")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Parse error response.
	var apiErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	if apiErr.Message != "" {
		return fmt.Errorf("API error: %s", apiErr.Message)
	}
	return fmt.Errorf("API returned %d", resp.StatusCode)
}
