//go:build integration

package wiring_test

import (
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mumia/wallet-accountant/internal/apitest"
)

func TestUnknownRouteIsProblemJSON(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	res := app.Get(t, "/api/no-such-route", "")

	res.AssertProblem(t, http.StatusNotFound, "about:blank")
}

// The API lives under /api, so the UI can own every other path of the same
// origin (ADR-0021). An API path without the prefix is not the API.
func TestAPIPathWithoutTheAPIPrefixIsNotFound(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.Get(t, "/me", "").AssertProblem(t, http.StatusNotFound, "about:blank")
	app.Get(t, "/api/me", "").AssertProblem(t, http.StatusUnauthorized, "about:blank")
}

// The chart's probes ask /healthz, outside the API and without a token:
// the Gateway never routes it to the app, only the kubelet asks it.
func TestHealthzAnswersWithoutAToken(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	if res := app.Get(t, "/healthz", ""); res.Status != http.StatusOK {
		t.Errorf("GET /healthz = %d %s, want 200", res.Status, res.Body)
	}

	if res := app.Do(t, http.MethodPost, "/healthz", "", nil); res.Status == http.StatusOK {
		t.Errorf("POST /healthz = 200, want only GET (and HEAD) answered")
	}
}

func TestWrongMethodIsProblemJSONWithAllow(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	res := app.Do(t, http.MethodDelete, "/api/me", "", nil)

	res.AssertProblem(t, http.StatusMethodNotAllowed, "about:blank")

	if allow := res.Header.Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Errorf("Allow = %q, want it to list GET", allow)
	}
}

func TestEveryRequestProducesALogRecordATraceAndMetrics(t *testing.T) {
	t.Parallel()
	app := apitest.Start(t)

	app.Get(t, "/api/me", "")

	spans := app.Spans.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no span recorded for the request")
	}

	traceID := spans[0].SpanContext.TraceID().String()

	var logged bool

	for _, rec := range app.Logs.Records(t) {
		if rec["msg"] == "http request" && rec["trace_id"] == traceID && rec["url.path"] == "/api/me" {
			logged = true
		}
	}

	if !logged {
		t.Errorf("no stdout JSON log record for the request in trace %s", traceID)
	}

	var bridged bool

	for _, rec := range app.OTLPLogs.Records() {
		if rec.Body().AsString() == "http request" && rec.TraceID().String() == traceID {
			bridged = true
		}
	}

	if !bridged {
		t.Errorf("no OTLP log record for the request in trace %s", traceID)
	}

	var rm metricdata.ResourceMetrics
	if err := app.Metrics.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}

	if !hasMetric(rm, "http.server.request.duration") {
		t.Error("no http.server.request.duration metric recorded")
	}
}

func hasMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return true
			}
		}
	}

	return false
}
