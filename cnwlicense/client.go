package cnwlicense

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout   = 10 * time.Second
	maxResponseBytes = 1 << 20 // 1 MB
)

// OnlineClient communicates with the CNW License Server HTTP API.
type OnlineClient struct {
	serverURL   string
	apiKey      string
	httpClient  *http.Client
	timeout     time.Duration // applied after all options
	userAgent   string
	fingerprint string
}

// NewOnlineClient creates a new client for the CNW License Server.
// serverURL is the base URL (e.g. "https://license.example.com").
// apiKey is the X-API-Key used for authentication.
func NewOnlineClient(serverURL, apiKey string, opts ...ClientOption) *OnlineClient {
	c := &OnlineClient{
		serverURL: strings.TrimRight(serverURL, "/"),
		apiKey:    apiKey,
		timeout:   defaultTimeout,
		userAgent: "cnw-license-sdk-go/1.0",
	}
	for _, opt := range opts {
		opt(c)
	}
	// If no custom HTTP client was provided, create one.
	// Apply timeout after all options so ordering doesn't matter.
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	c.httpClient.Timeout = c.timeout
	return c
}

// Fingerprint returns the fingerprint configured via WithFingerprint.
// Returns an empty string if no fingerprint was set.
func (c *OnlineClient) Fingerprint() string {
	return c.fingerprint
}

// Validate checks whether a license key is valid.
// The server returns the response directly (not wrapped in {data: ...}).
// If req.Fingerprint is empty and a client-level fingerprint is set via WithFingerprint,
// it is automatically used.
func (c *OnlineClient) Validate(ctx context.Context, req ValidateRequest) (*ValidateResponse, error) {
	if req.Fingerprint == "" && c.fingerprint != "" {
		req.Fingerprint = c.fingerprint
	}
	var resp ValidateResponse
	if err := c.doJSON(ctx, "/v1/validate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Activate registers a machine activation for a license key.
// The server wraps the response in {data: ...}.
// If req.Fingerprint is empty and a client-level fingerprint is set via WithFingerprint,
// it is automatically used.
func (c *OnlineClient) Activate(ctx context.Context, req ActivateRequest) (*ActivateResponse, error) {
	if req.Fingerprint == "" && c.fingerprint != "" {
		req.Fingerprint = c.fingerprint
	}
	var wrapper struct {
		Data ActivateResponse `json:"data"`
	}
	if err := c.doJSON(ctx, "/v1/activate", req, &wrapper); err != nil {
		return nil, err
	}
	return &wrapper.Data, nil
}

// doJSON performs a POST request with JSON body and decodes the response into dest.
// On non-2xx responses, it parses the server error format and returns a mapped error.
func (c *OnlineClient) doJSON(ctx context.Context, path string, body, dest interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return c.parseError(resp.StatusCode, respBody)
	}

	if err := json.Unmarshal(respBody, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// parseError parses the server error response format:
// {"error": {"code": "...", "message": "..."}}
func (c *OnlineClient) parseError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err != nil {
		return &ServerError{
			StatusCode: statusCode,
			Code:       "UNKNOWN",
			Message:    string(body),
		}
	}
	se := &ServerError{
		StatusCode: statusCode,
		Code:       errResp.Error.Code,
		Message:    errResp.Error.Message,
	}
	return mapServerError(se)
}
