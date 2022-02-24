// Package utclient implements uptime tracker client
package utclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/skycoin/dmsg/cipher"
	dmsgnetutil "github.com/skycoin/dmsg/netutil"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/internal/httpauth"
)

//go:generate mockery -name APIClient -case underscore -inpkg

// Error is the object returned to the client when there's an error.
type Error struct {
	Error string `json:"error"`
}

// APIClient implements uptime tracker API client.
type APIClient interface {
	UpdateVisorUptime(context.Context) error
}

// httpClient implements Client for uptime tracker API.
type httpClient struct {
	client *httpauth.Client
	pk     cipher.PubKey
	sk     cipher.SecKey
	log    *logging.Logger
}

const (
	createRetryDelay = 5 * time.Second
)

// NewHTTP creates a new client setting a public key to the client to be used for auth.
// When keys are set, the client will sign request before submitting.
// The signature information is transmitted in the header using:
// * SW-Public: The specified public key
// * SW-Nonce:  The nonce for that public key
// * SW-Sig:    The signature of the payload + the nonce
func NewHTTP(addr string, pk cipher.PubKey, sk cipher.SecKey, httpC *http.Client, clientPublicIP string, mLogger *logging.MasterLogger) (APIClient, error) {
	var client *httpauth.Client
	var err error

	log := mLogger.PackageLogger("utclient")

	retrier := dmsgnetutil.NewRetrier(log, createRetryDelay, 0, 10, 2)
	retrierFunc := func() error {
		client, err = httpauth.NewClient(context.Background(), addr, pk, sk, httpC, clientPublicIP, mLogger)
		if err != nil {
			return fmt.Errorf("uptime tracker httpauth: %w", err)
		}
		return nil
	}

	if err := retrier.Do(context.Background(), retrierFunc); err != nil {
		return nil, err
	}

	httpClient := &httpClient{
		client: client,
		pk:     pk,
		sk:     sk,
		log:    log,
	}

	return httpClient, nil
}

// Get performs a new GET request.
func (c *httpClient) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.client.Addr()+path, new(bytes.Buffer))
	if err != nil {
		return nil, err
	}

	return c.client.Do(req.WithContext(ctx))
}

// UpdateVisorUptime updates visor uptime.
func (c *httpClient) UpdateVisorUptime(ctx context.Context) error {
	resp, err := c.Get(ctx, "/v4/update")
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.log.WithError(err).Warn("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status: %d, error: %w", resp.StatusCode, extractError(resp.Body))
	}

	return nil
}

// extractError returns the decoded error message from Body.
func extractError(r io.Reader) error {
	var apiError Error

	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, &apiError); err != nil {
		return errors.New(string(body))
	}

	return errors.New(apiError.Error)
}
