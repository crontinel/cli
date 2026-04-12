# Crontinel CLI

Monitor your Laravel cron jobs and queue workers from the terminal.

```bash
# Install (macOS/Linux)
curl -sSL https://get.crontinel.com/install | sh

# Or download from releases
curl -L https://github.com/crontinel/cli/releases/latest/download/crontinel -o crontinel
chmod +x crontinel
mv crontinel /usr/local/bin/
```

```bash
# Set your API key
export CRONTINEL_API_KEY=your_key_here

# Verify connectivity
crontinel ping

# List monitors
crontinel monitors

# View recent events
crontinel events

# Check alert channels
crontinel alerts

# JSON output for scripting
crontinel monitors --json
```

## Commands

| Command | Description |
|---------|-------------|
| `ping`, `health` | Test your connection to Crontinel |
| `monitors`, `list` | Show all configured monitors |
| `events` | View recent firing/resolved events |
| `alerts` | List configured alert channels |

## Options

- `--key <key>` — API key (or set `CRONTINEL_API_KEY`)
- `--url <url>` — API URL (default: `https://app.crontinel.com`)
- `--json` — Raw JSON output

## Requirements

- API key from [app.crontinel.com](https://app.crontinel.com)
- Internet connectivity
