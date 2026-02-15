package cnwlicense

import (
	"net/http"
	"time"
)

// ClientOption configures an OnlineClient.
type ClientOption func(*OnlineClient)

// WithHTTPClient sets a custom HTTP client for the OnlineClient.
// The client's Timeout will be overridden by WithTimeout (or the default 10s).
func WithHTTPClient(c *http.Client) ClientOption {
	return func(o *OnlineClient) {
		o.httpClient = c
	}
}

// WithTimeout sets the HTTP client timeout. Default is 10 seconds.
// Option ordering does not matter: timeout is always applied after all options.
func WithTimeout(d time.Duration) ClientOption {
	return func(o *OnlineClient) {
		o.timeout = d
	}
}

// WithUserAgent sets the User-Agent header sent with requests.
func WithUserAgent(ua string) ClientOption {
	return func(o *OnlineClient) {
		o.userAgent = ua
	}
}
