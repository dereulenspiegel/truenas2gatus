package truenas

import (
	"encoding/json"
	"net/http"
)

type PoolStatus string

const (
	PoolStatusOnline PoolStatus = "ONLINE"
)

type Pool struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	GUID         string          `json:"guid"`
	Path         string          `json:"path"`
	Status       PoolStatus      `json:"status"`
	Errors       int             `json:"errors"`
	Healthy      bool            `json:"healthy"`
	Warning      bool            `json:"warning"`
	StatusCode   string          `json:"status_code"`
	StatusDetail json.RawMessage `json:"status_detail"`
}

func (c *Client) GetPools() ([]Pool, error) {
	pools := make([]Pool, 0)
	req, err := http.NewRequest(http.MethodGet, c.buildUrl("/pool"), nil)
	if err != nil {
		return nil, &TrueNasError{Reason: err}
	}
	if err := do(c, req, &pools); err != nil {
		return nil, err
	}
	return pools, err
}

func IsPoolHealthy(pool Pool) bool {
	if pool.Status != PoolStatusOnline {
		return false
	}
	if !pool.Healthy {
		return false
	}
	if pool.Warning {
		return false
	}
	if pool.Errors > 0 {
		return false
	}
	return true
}
