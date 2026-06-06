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
		if req.Method != "list/jobs" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}
		if got := req.Params["take"]; got != float64(50) {
			t.Fatalf("unexpected take param: %#v", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"monitors":[{"name":"Nightly sync","schedule":"0 2 * * *"},{"name":"Paused job","schedule":"*/5 * * * *","is_paused":true}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdMonitors(false); err != nil {
			t.Fatalf("cmdMonitors returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Monitors (2):") {
		t.Fatalf("expected monitor count in output, got: %s", output)
	}
	if !strings.Contains(output, "● Nightly sync (0 2 * * *)") {
		t.Fatalf("expected active monitor in output, got: %s", output)
	}
	if !strings.Contains(output, "⏸ Paused job (*/5 * * * *)") {
		t.Fatalf("expected paused monitor in output, got: %s", output)
	}
}

func TestCmdEventsReportsStateIcons(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "list/events" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}
		if got := req.Params["take"]; got != float64(20) {
			t.Fatalf("unexpected take param: %#v", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"events":[{"state":"firing","message":"Queue worker stalled","created_at":"2026-06-06T00:00:00Z"},{"state":"resolved","message":"Cron recovered","created_at":"2026-06-06T01:00:00Z"}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdEvents(false); err != nil {
			t.Fatalf("cmdEvents returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Recent events (2):") {
		t.Fatalf("expected event count in output, got: %s", output)
	}
	if !strings.Contains(output, "✗ Queue worker stalled [2026-06-06T00:00:00Z]") {
		t.Fatalf("expected firing event in output, got: %s", output)
	}
	if !strings.Contains(output, "✓ Cron recovered [2026-06-06T01:00:00Z]") {
		t.Fatalf("expected resolved event in output, got: %s", output)
	}
}

func TestCmdAlertsListsChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Method != "list/alerts" {
			t.Fatalf("unexpected RPC method: %s", req.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"channels":[{"type":"email"},{"type":"slack"}]}}`))
	}))
	defer server.Close()

	withTestAPI(t, server)

	output := captureOutput(t, func() {
		if err := cmdAlerts(false); err != nil {
			t.Fatalf("cmdAlerts returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Alert channels (2):") {
		t.Fatalf("expected alert count in output, got: %s", output)
	}
	if !strings.Contains(output, "• email") {
		t.Fatalf("expected email alert channel in output, got: %s", output)
	}
	if !strings.Contains(output, "• slack") {
		t.Fatalf("expected slack alert channel in output, got: %s", output)
	}
}
