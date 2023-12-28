package truenas

import (
	"crypto/tls"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationGetPools(t *testing.T) {
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	apiKey := os.Getenv("TRUENAS_API_KEY")
	host := os.Getenv("TRUENAS_HOST")

	c, err := NewClient(host, apiKey, WithHttpClient(&httpClient))
	require.NoError(t, err)
	pools, err := c.GetPools()
	require.NoError(t, err)
	assert.NotNil(t, pools)
	assert.Len(t, pools, 1)
}
