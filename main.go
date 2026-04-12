package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	apiKey string
	apiURL string
)

func main() {
	if err := run(os.Args); err != nil {
		if err == flag.ErrHelp || err == context.DeadlineExceeded {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		printUsage()
		return flag.ErrHelp
	}

	fs := flag.NewFlagSet("crontinel", flag.ContinueOnError)
	fs.Usage = func() { printUsage() }

	apiKeyPtr := fs.String("key", "", "Crontinel API key (or CRONTINEL_API_KEY env)")
	apiURLPtr := fs.String("url", "https://app.crontinel.com", "Crontinel API URL")
	jsonPtr := fs.Bool("json", false, "Output JSON")

	fs.Parse(args[2:])

	if *apiKeyPtr != "" {
		apiKey = *apiKeyPtr
	} else if key := os.Getenv("CRONTINEL_API_KEY"); key != "" {
		apiKey = key
	} else {
		return fmt.Errorf("API key required: set CRONTINEL_API_KEY env var or use --key flag")
	}
	apiURL = *apiURLPtr

	cmd := args[1]
	var err error

	switch cmd {
	case "ping", "health":
		err = cmdPing(*jsonPtr)
	case "monitors", "list":
		err = cmdMonitors(*jsonPtr)
	case "events":
		err = cmdEvents(*jsonPtr)
	case "alerts":
		err = cmdAlerts(*jsonPtr)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		return flag.ErrHelp
	}
	return err
}

func printUsage() {
	fmt.Printf(`Crontinel CLI — monitor your background jobs and cron

Usage:
  crontinel [options] <command>

Commands:
  ping, health       Send a ping to verify connectivity
  monitors, list     List all monitors
  events             List recent events
  alerts             List configured alert channels

Options:
  --key <key>        API key (or CRONTINEL_API_KEY env)
  --url <url>        API URL (default: https://app.crontinel.com)
  --json             Output raw JSON response

Examples:
  crontinel ping
  CRONTINEL_API_KEY=xxx crontinel monitors --json
  crontinel events --key xxx

`)
}

type RPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      int                    `json:"id"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func doRPC(ctx context.Context, method string, params map[string]interface{}) (*RPCResponse, error) {
	body, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/mcp", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var rpcResp RPCResponse
	if err := json.Unmarshal(bodyBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}

func cmdPing(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doRPC(ctx, "list/jobs", map[string]interface{}{"take": 1})
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)
	fmt.Println("✓ Connected to Crontinel")
	if mon, ok := result["monitors"].([]any); ok && len(mon) > 0 {
		fmt.Printf("  Monitors: %d\n", len(mon))
	}
	return nil
}

func cmdMonitors(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doRPC(ctx, "list/jobs", map[string]interface{}{"take": 50})
	if err != nil {
		return fmt.Errorf("failed to list monitors: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)
	monitors, _ := result["monitors"].([]any)

	if len(monitors) == 0 {
		fmt.Println("No monitors found. Add one at app.crontinel.com")
		return nil
	}

	fmt.Printf("Monitors (%d):\n", len(monitors))
	for _, m := range monitors {
		mon := m.(map[string]interface{})
		status := "●"
		if mon["is_paused"] == true {
			status = "⏸"
		}
		fmt.Printf("  %s %s (%s)\n", status, mon["name"], mon["schedule"])
	}
	return nil
}

func cmdEvents(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doRPC(ctx, "list/events", map[string]interface{}{"take": 20})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)
	events, _ := result["events"].([]any)

	if len(events) == 0 {
		fmt.Println("No recent events.")
		return nil
	}

	fmt.Printf("Recent events (%d):\n", len(events))
	for _, e := range events {
		ev := e.(map[string]interface{})
		state := ev["state"]
		icon := "○"
		if state == "firing" {
			icon = "✗"
		} else if state == "resolved" {
			icon = "✓"
		}
		ts := ev["created_at"]
		msg := ev["message"]
		fmt.Printf("  %s %s [%s]\n", icon, msg, ts)
	}
	return nil
}

func cmdAlerts(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doRPC(ctx, "list/alerts", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to list alerts: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)
	alerts, _ := result["channels"].([]any)

	if len(alerts) == 0 {
		fmt.Println("No alert channels configured. Add one at app.crontinel.com")
		return nil
	}

	fmt.Printf("Alert channels (%d):\n", len(alerts))
	for _, a := range alerts {
		al := a.(map[string]interface{})
		typ := al["type"]
		fmt.Printf("  • %s\n", typ)
	}
	return nil
}
