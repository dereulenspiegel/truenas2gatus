package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dereulenspiegel/truenas2gatos/gatus"
	"github.com/dereulenspiegel/truenas2gatos/truenas"
)

type store interface {
	SaveResult(result *gatus.Result) error
	GetResults() ([]*gatus.Result, error)
}

const (
	ENV_TRUENAS_HOSTNAME     = "TRUENAS_HOST"
	ENV_TRUENAS_API_KEY      = "TRUENAS_API_KEY"
	ENV_TRUENAS_INTERVAL     = "TRUENAS_INTERVAL"
	ENV_TRUENAS_TRUST_ALL    = "TRUENAS_TLS_TRUST_ALL"
	ENV_TRUENAS_RESULT_STORE = "TRUENAS_RESULT_STORE"
)

var (
	defaultInterval = time.Minute
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	logger := slog.Default()

	logger.Info("Starting truenas2gatus")
	ctx, backgroundCancel := context.WithCancel(context.Background())
	defer backgroundCancel()
	go func() {

		var err error
		host := os.Getenv(ENV_TRUENAS_HOSTNAME)
		apiKey := os.Getenv(ENV_TRUENAS_API_KEY)
		intervalStr := os.Getenv(ENV_TRUENAS_INTERVAL)
		var interval time.Duration
		if intervalStr == "" {
			interval = defaultInterval
		} else {
			interval, err = time.ParseDuration(intervalStr)
			if err != nil {
				logger.Error("failed to parse interval", "err", err, "interval", intervalStr)
				os.Exit(1)
			}
		}
		trustAllCerts, err := strconv.ParseBool(os.Getenv(ENV_TRUENAS_TRUST_ALL))
		if err != nil {
			trustAllCerts = false
		}
		httpClient := http.DefaultClient
		if trustAllCerts {
			httpClient = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
		}

		trueNasHostUrl, err := url.Parse(host)
		if err != nil {
			logger.Error("invalid true nas url", "err", err, "trueNasUrl", host)
		}

		trueNasClient, err := truenas.NewClient(host, apiKey, truenas.WithHttpClient(httpClient))
		if err != nil {
			logger.Error("failed to create true nas client", "err", err)
			os.Exit(1)
		}

		var st store
		storePath := os.Getenv(ENV_TRUENAS_RESULT_STORE)
		if storePath == "" {
			storePath = "/data/results.json"
		}
		st, err = NewFileStore(storePath, 20)
		if err != nil {
			logger.Error("failed to create data store", "err", err)
			os.Exit(1)
		}

		trueNasCtx := context.WithValue(ctx, "ctxName", "trueNas")
		go func(ctx context.Context) {
			logger.Info("Starting timer for interval", "interval", interval)
			queryTimer := time.NewTimer(interval)
			for {
				select {
				case <-ctx.Done():
					logger.Info("TrueNAS query routine cancelled", "cause", context.Cause(ctx))
					return
				case <-queryTimer.C:
					logger.Info("Querying TrueNAS Pool status")
					queryAndSave(logger, trueNasClient, st, trueNasHostUrl)
				}
			}
		}(trueNasCtx)

		mux := http.NewServeMux()
		mux.HandleFunc("/results", func(w http.ResponseWriter, r *http.Request) {
			results, err := st.GetResults()
			if err != nil {
				logger.Error("failed to retrieve results", "err", err)
				http.Error(w, "data store failure", http.StatusInternalServerError)
				return
			}

			endpointStatus := []*gatus.EndpointStatus{
				{
					Name:    "TrueNAS",
					Group:   "Storage",
					Key:     "storage_truenas",
					Results: results,
				},
			}
			if err := json.NewEncoder(w).Encode(endpointStatus); err != nil {
				logger.Error("failed to marshal results in http response", "err", err)
			}
		})
		if err := http.ListenAndServe(":8989", mux); err != nil && err != http.ErrServerClosed {
			logger.Error("failure on http server", "err", err)
		}
	}()

	<-sigs
	logger.Info("Exiting application")
}

func queryAndSave(logger *slog.Logger, trueNasClient *truenas.Client, st store, trueNasHostUrl *url.URL) {
	start := time.Now()
	pools, err := trueNasClient.GetPools()
	end := time.Since(start)
	var result *gatus.Result
	if err != nil {
		logger.Error("failed to query TrueNAS pools", "err", err)
		httpStatus := 0
		var trueNasErr *truenas.TrueNasError
		if errors.As(err, &trueNasErr) {
			httpStatus = trueNasErr.StatusCode
		}
		result = &gatus.Result{
			HTTPStatus: httpStatus,
			Hostname:   trueNasHostUrl.Host,
			Duration:   end,
			Errors:     []string{err.Error()},
			Success:    false,
			Timestamp:  time.Now(),
			ConditionResults: []*gatus.ConditionResult{
				{
					Success:   false,
					Condition: "Connected == false",
				},
			},
		}
	} else {
		result = &gatus.Result{
			HTTPStatus: 0,
			Hostname:   trueNasHostUrl.Host,
			Duration:   end,
		}
		overallFailure := false
		var conditions []*gatus.ConditionResult
		for _, pool := range pools {
			if !truenas.IsPoolHealthy(pool) {
				conditions = append(conditions, &gatus.ConditionResult{
					Condition: fmt.Sprintf("%s == Healthy", pool.Name),
					Success:   false,
				})
				overallFailure = true
			} else {
				conditions = append(conditions, &gatus.ConditionResult{
					Condition: fmt.Sprintf("%s == Healthy", pool.Name),
					Success:   true,
				})
			}
		}
		result.Success = !overallFailure
		result.ConditionResults = conditions
		result.Duration = end
		result.Timestamp = time.Now()
	}
	logger.Info("Saving result from TrueNAS pools", "success", result.Success)
	if err := st.SaveResult(result); err != nil {
		logger.Error("failed to store result", "err", err)
	}
}
