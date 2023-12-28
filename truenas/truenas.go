package truenas

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	client   *http.Client
	host     string
	apiToken string
}

type TrueNasError struct {
	Reason        error
	StatusCode    int
	StatusMessage string
}

func (t *TrueNasError) Error() string {
	if t.Reason != nil {
		return t.Reason.Error()
	}
	return fmt.Sprintf("[%d] %s", t.StatusCode, t.StatusMessage)
}

type clientOpt func(*Client)

func WithHttpClient(httpClient *http.Client) clientOpt {
	return func(c *Client) {
		c.client = httpClient
	}
}

func NewClient(host, apiToken string, opts ...clientOpt) (*Client, error) {
	c := &Client{
		host:     host,
		apiToken: apiToken,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.client == nil {
		c.client = http.DefaultClient
	}
	return c, nil
}

func (c *Client) buildUrl(requestPath string) string {
	return fmt.Sprintf("%s/api/v2.0%s", c.host, requestPath)
}

func do[T any](c *Client, req *http.Request, res T) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	resp, err := c.client.Do(req)
	if err != nil {
		return &TrueNasError{
			Reason: err,
		}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TrueNasError{
			Reason: err,
		}
	}
	if resp.StatusCode >= 400 {
		return &TrueNasError{
			StatusCode:    resp.StatusCode,
			StatusMessage: string(body), // TODO parse error messages
		}
	}
	if err := json.Unmarshal(body, res); err != nil {
		return &TrueNasError{
			Reason: err,
		}
	}
	return nil
}
