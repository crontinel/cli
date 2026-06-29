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

	cmd := args[1]
	flagStart := 2

	// Check if args[1] is a flag (--xxx) vs a command
	if len(cmd) > 0 && cmd[0] == '-' {
		// Flags before command: parse from args[1:], command is first non-flag arg
		fs2 := flag.NewFlagSet("crontinel", flag.ContinueOnError)
		fs2.Usage = func() { printUsage() }
		apiKeyPtr2 := fs2.String("key", "", "Crontinel API key (or CRONTINEL_API_KEY env)")
		apiURLPtr2 := fs2.String("url", "https://app.crontinel.com", "Crontinel API URL")
		jsonPtr2 := fs2.Bool("json", false, "Output JSON")
		if err := fs2.Parse(args[1:]); err != nil {
			return err
		}
		if fs2.NArg() == 0 {
			printUsage()
			return nil
		}
		cmd = fs2.Arg(0)
		apiKeyPtr = apiKeyPtr2
		apiURLPtr = apiURLPtr2
		jsonPtr = jsonPtr2
	} else {
		// Command before flags: parse from args[2:]
		if err := fs.Parse(args[flagStart:]); err != nil {
			return err
		}
	}

	// Help for --help flag
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		printUsage()
		return nil
	}

	// Validate command before requiring API key
	validCmds := map[string]bool{"ping": true, "health": true, "monitors": true, "list": true, "events": true, "alerts": true}
	if !validCmds[cmd] {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		return flag.ErrHelp
	}

	// Resolve API key
	if *apiKeyPtr != "" {
		apiKey = *apiKeyPtr
	} else if key := os.Getenv("CRONTINEL_API_KEY"); key != "" {
		apiKey = key
	} else {
		return fmt.Errorf("API key required: set CRONTINEL_API_KEY env var or use --key flag")
	}
	apiURL = *apiURLPtr

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

func doCall(ctx context.Context, tool string, args map[string]interface{}) (*RPCResponse, error) {
	body, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      tool,
			"arguments": args,
		},
		ID: 1,
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

	body, _ := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/mcp", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var rpcResp RPCResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("ping failed: RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if jsonOutput {
		fmt.Println(string(rpcResp.Result))
		return nil
	}

	var result struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	json.Unmarshal(rpcResp.Result, &result)
	fmt.Println("✓ Connected to Crontinel")
	fmt.Printf("  Available tools: %d\n", len(result.Tools))
	return nil
}

func cmdMonitors(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doCall(ctx, "list_scheduled_jobs", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to list monitors: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	text, err := extractMCPText(resp)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	var runs []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &runs); err != nil {
		return fmt.Errorf("failed to parse job list: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No scheduled jobs found. Use the app to create one.")
		return nil
	}

	fmt.Printf("Scheduled jobs (%d):\n", len(runs))
	for _, job := range runs {
		cmd := job["command"]
		status := job["last_status"]
		icon := "●"
		if status == "failed" || status == "error" {
			icon = "✗"
		}
		fmt.Printf("  %s %s (last: %s)\n", icon, cmd, status)
	}
	return nil
}

func extractMCPText(resp *RPCResponse) (string, error) {
	var wrapper struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &wrapper); err != nil {
		return "", fmt.Errorf("failed to parse MCP response: %w", err)
	}
	if len(wrapper.Content) == 0 {
		return "", fmt.Errorf("empty MCP response")
	}
	return wrapper.Content[0].Text, nil
}

func cmdEvents(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doCall(ctx, "list_recent_alerts", map[string]interface{}{"hours": 72})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	text, err := extractMCPText(resp)
	if err != nil {
		return fmt.Errorf("failed to parse events: %w", err)
	}

	var alerts []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &alerts); err != nil {
		return fmt.Errorf("failed to parse alert list: %w", err)
	}

	if len(alerts) == 0 {
		fmt.Println("No recent alerts.")
		return nil
	}

	fmt.Printf("Recent alerts (%d):\n", len(alerts))
	for _, alert := range alerts {
		state := alert["state"]
		icon := "○"
		if state == "firing" || state == "active" {
			icon = "✗"
		} else if state == "resolved" {
			icon = "✓"
		}
		key := alert["alert_key"]
		ts := alert["fired_at"]
		fmt.Printf("  %s %s [%s]\n", icon, key, ts)
	}
	return nil
}

func cmdAlerts(jsonOutput bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := doCall(ctx, "list_recent_alerts", map[string]interface{}{"hours": 168})
	if err != nil {
		return fmt.Errorf("failed to list alerts: %w", err)
	}

	if jsonOutput {
		fmt.Println(string(resp.Result))
		return nil
	}

	text, err := extractMCPText(resp)
	if err != nil {
		return fmt.Errorf("failed to parse alerts: %w", err)
	}

	var alerts []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &alerts); err != nil {
		return fmt.Errorf("failed to parse alert list: %w", err)
	}

	if len(alerts) == 0 {
		fmt.Println("No alerts in the last 7 days.")
		return nil
	}

	fmt.Printf("Recent alerts (%d):\n", len(alerts))
	for _, al := range alerts {
		state := al["state"]
		icon := "○"
		if state == "firing" || state == "active" {
			icon = "⚠"
		} else if state == "resolved" {
			icon = "✓"
		}
		key := al["alert_key"]
		count := al["fire_count"]
		ts := al["fired_at"]
		fmt.Printf("  %s %s (×%v) [%s]\n", icon, key, count, ts)
	}
	return nil
}
