package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	output := <-outCh
	_ = r.Close()
	return output
}

func withTestAPI(t *testing.T, server *httptest.Server) {
	t.Helper()

	oldAPIURL := apiURL
	oldAPIKey := apiKey
	apiURL = server.URL
	apiKey = "test-key"
	t.Cleanup(func() {
		apiURL = oldAPIURL
		apiKey = oldAPIKey
	})
}

func TestCmdMonitorsReportsPausesAndSchedules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Path; got != "/api/mcp" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}

		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}
		params := req.Params
		if params["name"] != "list_scheduled_jobs" {
			t.Fatalf("unexpected tool name: %v", params["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"[{\"command\":\"Nightly sync\",\"last_status\":\"success\",\"last_run_at\":\"2026-06-28T02:00:00Z\",\"run_count_today\":1},{\"command\":\"Paused job\",\"last_status\":\"paused\",\"last_run_at\":\"2026-06-27T00:00:00Z\",\"run_count_today\":0}]"}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdMonitors(false); err != nil {
			t.Fatalf("cmdMonitors returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Scheduled jobs (2):") {
		t.Fatalf("expected job count in output, got: %s", output)
	}
	if !strings.Contains(output, "● Nightly sync (last: success)") {
		t.Fatalf("expected success job in output, got: %s", output)
	}
	if !strings.Contains(output, "● Paused job (last: paused)") {
		t.Fatalf("expected paused job in output, got: %s", output)
	}
}

func TestCmdEventsReportsStateIcons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}
		params := req.Params
		if params["name"] != "list_recent_alerts" {
			t.Fatalf("unexpected tool name: %v", params["name"])
		}
		if params["arguments"].(map[string]interface{})["hours"] != float64(72) {
			t.Fatalf("unexpected hours param: %#v", params["arguments"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"[{\"alert_key\":\"Queue worker stalled\",\"state\":\"firing\",\"fire_count\":3,\"fired_at\":\"2026-06-06T00:00:00Z\"},{\"alert_key\":\"Cron recovered\",\"state\":\"resolved\",\"fire_count\":1,\"fired_at\":\"2026-06-06T01:00:00Z\"}]"}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdEvents(false); err != nil {
			t.Fatalf("cmdEvents returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Recent alerts (2):") {
		t.Fatalf("expected alert count in output, got: %s", output)
	}
	if !strings.Contains(output, "✗ Queue worker stalled") {
		t.Fatalf("expected firing event in output, got: %s", output)
	}
	if !strings.Contains(output, "✓ Cron recovered") {
		t.Fatalf("expected resolved event in output, got: %s", output)
	}
}

func TestCmdAlertsListsChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "tools/call" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}
		params := req.Params
		if params["name"] != "list_recent_alerts" {
			t.Fatalf("unexpected tool name: %v", params["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"[{\"alert_key\":\"Queue stalled\",\"state\":\"firing\",\"fire_count\":5,\"fired_at\":\"2026-06-06T00:00:00Z\"},{\"alert_key\":\"Disk space\",\"state\":\"resolved\",\"fire_count\":2,\"fired_at\":\"2026-06-05T12:00:00Z\"}]"}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdAlerts(false); err != nil {
			t.Fatalf("cmdAlerts returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Recent alerts (2):") {
		t.Fatalf("expected alert count in output, got: %s", output)
	}
	if !strings.Contains(output, "⚠ Queue stalled") {
		t.Fatalf("expected firing alert in output, got: %s", output)
	}
	if !strings.Contains(output, "✓ Disk space") {
		t.Fatalf("expected resolved alert in output, got: %s", output)
	}
}
