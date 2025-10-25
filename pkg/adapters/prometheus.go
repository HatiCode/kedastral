// Package adapters provides Kedastral data source connectors that retrieve
// metrics or contextual signals from external systems and normalize them
// into a common DataFrame structure.
//
// Each adapter implements the Adapter interface and can be plugged into
// the Kedastral Forecast Engine. Typical adapters include:
//   - PrometheusAdapter — fetches metrics via the Prometheus HTTP API
//   - HTTPAdapter       — calls arbitrary REST endpoints for events or data
//   - ScheduleAdapter   — provides upcoming time-based events (e.g. matches)
//   - KafkaAdapter      — reads lag, queue depth, or message rate
//
// Adapters are intentionally lightweight. They focus on pulling raw data,
// shaping it into [DataFrame] objects, and leaving all feature building and
// forecasting logic to Kedastral’s upper layers.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// PrometheusAdapter fetches time-series data from the Prometheus HTTP API.
// It issues a /api/v1/query_range call and returns a *DataFrame with rows of the form:
//
//	{"ts": RFC3339 string, "value": float64}
//
// If multiple series are returned, values with the same timestamp are SUMMED.
type PrometheusAdapter struct {
	// ServerURL is the base URL to Prometheus, e.g. http://prometheus.monitoring.svc:9090
	ServerURL string
	// Query is the PromQL expression to evaluate.
	Query string
	// StepSeconds controls the resolution (defaults to 60s if <= 0).
	StepSeconds int
	// HTTPClient is optional; if nil a default client with timeout is used.
	HTTPClient *http.Client
}

func (p *PrometheusAdapter) Name() string { return "prometheus" }

// Collect implements Adapter. It queries Prometheus for the last windowSeconds worth
// of data, at StepSeconds resolution, and returns a *DataFrame. It respects the
// provided context for cancellation and deadlines.
func (p *PrometheusAdapter) Collect(ctx context.Context, windowSeconds int) (*DataFrame, error) {
	if p.ServerURL == "" || p.Query == "" {
		return &DataFrame{}, errors.New("prometheus adapter: ServerURL and QueryURL are required")
	}
	step := p.StepSeconds
	if step <= 0 {
		step = 60
	}
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-time.Duration(windowSeconds) * time.Second)

	u, err := url.Parse(p.ServerURL)
	if err != nil {
		return &DataFrame{}, fmt.Errorf("invalid ServerURL: %w", err)
	}

	q := u.Query()
	q.Set("query", p.Query)
	q.Set("start", fmt.Sprintf("%d", start.Unix()))
	q.Set("end", fmt.Sprintf("%d", now.Unix()))
	q.Set("step", fmt.Sprintf("%d", step))
	u.RawQuery = q.Encode()

	cli := p.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return &DataFrame{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return &DataFrame{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &DataFrame{}, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}

	var pr prometheusRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return &DataFrame{}, fmt.Errorf("decode prometheus response: %w", err)
	}
	if pr.Status != "success" {
		return &DataFrame{}, fmt.Errorf("prometheus status: %s", pr.Status)
	}

	rows, err := aggregateRangeResult(pr.Data.Result)
	if err != nil {
		return &DataFrame{}, err
	}

	// Ensure sorted by timestamp
	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["ts"].(time.Time).Before(rows[j]["ts"].(time.Time))
	})

	for i := range rows {
		rows[i]["ts"] = rows[i]["ts"].(time.Time).UTC().Format(time.RFC3339)
	}

	return &DataFrame{Rows: rows}, nil
}

type prometheusRangeResponse struct {
	Status string              `json:"status"`
	Data   prometheusRangeData `json:"data"`
}

type prometheusRangeData struct {
	ResultType string                 `json:"resultType"`
	Result     []prometheusRangeSerie `json:"result"`
}

type prometheusRangeSerie struct {
	Metric map[string]string `json:"metric"`
	// Values is an array of [ <unix_time_float>, "<value_string>" ]
	Values [][]any `json:"values"`
}

func aggregateRangeResult(series []prometheusRangeSerie) ([]Row, error) {
	acc := make(map[int64]float64)
	for _, s := range series {
		for _, pair := range s.Values {
			if len(pair) != 2 {
				return nil, fmt.Errorf("invalid value pair length: %d", len(pair))
			}

			var tsSec int64
			switch v := pair[0].(type) {
			case float64:
				tsSec = int64(v)
			case json.Number:
				f, _ := v.Float64()
				tsSec = int64(f)
			default:
				return nil, fmt.Errorf("unexpected timestamp type %T", v)
			}

			var val float64
			switch vv := pair[1].(type) {
			case string:
				f, err := strconv.ParseFloat(vv, 64)
				if err != nil {
					return nil, fmt.Errorf("parse value: %w", err)
				}
				val = f
			case float64:
				val = vv
			case json.Number:
				f, _ := vv.Float64()
				val = f
			default:
				return nil, fmt.Errorf("unexpected value type %T", vv)
			}
			acc[tsSec] += val
		}
	}

	rows := make([]Row, 0, len(acc))
	for ts, v := range acc {
		rows = append(rows, Row{
			"ts":    time.Unix(ts, 0).UTC(),
			"value": v,
		})
	}
	return rows, nil
}
