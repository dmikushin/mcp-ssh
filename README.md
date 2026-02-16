# mcp-ssh

MCP server that wraps native `ssh`/`scp` binaries. Inherits ControlMaster multiplexing, ProxyJump, agent forwarding, and everything else from `~/.ssh/config` for free.

## Why

The existing SSH MCP servers use JS SSH libraries that don't read `~/.ssh/config`, don't support ProxyJump, and reconnect for every command. This server just calls `ssh` and `scp` via `os/exec`, so it gets all your SSH config features automatically.

## Tools

| Tool | Description |
|------|-------------|
| `ssh_list_hosts` | List all hosts from `~/.ssh/config` with Hostname, User, Port |
| `ssh_run` | Run a command on a remote host by config alias |
| `ssh_run_batch` | Run multiple commands sequentially in a single SSH session (`&&`-joined) |
| `ssh_upload` | Upload a file via `scp` |
| `ssh_download` | Download a file via `scp` |
| `ssh_check` | Test connectivity to a host (`ssh -o ConnectTimeout=5 host true`) |

## Build

```
go build -o mcp-ssh .
```

## Claude Code integration

Add to `~/.claude.json` under `mcpServers`:

```json
"mcp-ssh": {
  "command": "/path/to/mcp-ssh"
}
```

## Dependencies

- [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official MCP Go SDK
- [github.com/kevinburke/ssh_config](https://github.com/kevinburke/ssh_config) — SSH config parser
