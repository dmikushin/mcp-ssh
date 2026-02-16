package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool input types

type ListHostsInput struct{}

type RunInput struct {
	Host    string `json:"host" jsonschema:"SSH config alias (e.g. oracle, gambetta)"`
	Command string `json:"command" jsonschema:"Shell command to execute"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)"`
}

type RunBatchInput struct {
	Host     string   `json:"host" jsonschema:"SSH config alias"`
	Commands []string `json:"commands" jsonschema:"Commands to run sequentially (joined with &&)"`
	Timeout  int      `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)"`
}

type UploadInput struct {
	Host       string `json:"host" jsonschema:"SSH config alias"`
	LocalPath  string `json:"local_path" jsonschema:"Local file path"`
	RemotePath string `json:"remote_path" jsonschema:"Remote destination path"`
	Timeout    int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)"`
}

type DownloadInput struct {
	Host       string `json:"host" jsonschema:"SSH config alias"`
	RemotePath string `json:"remote_path" jsonschema:"Remote file path"`
	LocalPath  string `json:"local_path" jsonschema:"Local destination path"`
	Timeout    int    `json:"timeout,omitempty" jsonschema:"Timeout in seconds (default 30)"`
}

type CheckInput struct {
	Host string `json:"host" jsonschema:"SSH config alias to test connectivity"`
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}, nil, nil
}

func errorResult(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, nil, nil
}

func execResultToText(r *ExecResult) string {
	var sb strings.Builder
	if r.Stdout != "" {
		sb.WriteString(r.Stdout)
	}
	if r.Stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr] ")
		sb.WriteString(r.Stderr)
	}
	if r.ExitCode != 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[exit code: %d]", r.ExitCode))
	}
	if sb.Len() == 0 {
		sb.WriteString("(no output)")
	}
	return sb.String()
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-ssh",
		Version: "1.0.0",
	}, nil)

	// ssh_list_hosts
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_list_hosts",
		Description: "List all hosts from ~/.ssh/config with their key properties (Hostname, User, Port)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListHostsInput) (*mcp.CallToolResult, any, error) {
		hosts, err := ListHosts()
		if err != nil {
			return errorResult(fmt.Sprintf("Failed to list hosts: %v", err))
		}
		data, _ := json.MarshalIndent(hosts, "", "  ")
		return textResult(string(data))
	})

	// ssh_run
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_run",
		Description: "Run a command on a remote host via SSH config alias",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, any, error) {
		if in.Host == "" || in.Command == "" {
			return errorResult("host and command are required")
		}
		result, err := sshRun(ctx, in.Host, in.Command, in.Timeout)
		if err != nil {
			return errorResult(fmt.Sprintf("SSH error: %v", err))
		}
		return textResult(execResultToText(result))
	})

	// ssh_run_batch
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_run_batch",
		Description: "Run multiple commands sequentially on one host in a single SSH session (joined with &&)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in RunBatchInput) (*mcp.CallToolResult, any, error) {
		if in.Host == "" || len(in.Commands) == 0 {
			return errorResult("host and commands are required")
		}
		result, err := sshRunBatch(ctx, in.Host, in.Commands, in.Timeout)
		if err != nil {
			return errorResult(fmt.Sprintf("SSH error: %v", err))
		}
		return textResult(execResultToText(result))
	})

	// ssh_upload
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_upload",
		Description: "Upload a file to a remote host via scp",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in UploadInput) (*mcp.CallToolResult, any, error) {
		if in.Host == "" || in.LocalPath == "" || in.RemotePath == "" {
			return errorResult("host, local_path, and remote_path are required")
		}
		result, err := scpUpload(ctx, in.Host, in.LocalPath, in.RemotePath, in.Timeout)
		if err != nil {
			return errorResult(fmt.Sprintf("SCP error: %v", err))
		}
		if result.ExitCode != 0 {
			return errorResult(execResultToText(result))
		}
		return textResult(fmt.Sprintf("Uploaded %s to %s:%s", in.LocalPath, in.Host, in.RemotePath))
	})

	// ssh_download
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_download",
		Description: "Download a file from a remote host via scp",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in DownloadInput) (*mcp.CallToolResult, any, error) {
		if in.Host == "" || in.RemotePath == "" || in.LocalPath == "" {
			return errorResult("host, remote_path, and local_path are required")
		}
		result, err := scpDownload(ctx, in.Host, in.RemotePath, in.LocalPath, in.Timeout)
		if err != nil {
			return errorResult(fmt.Sprintf("SCP error: %v", err))
		}
		if result.ExitCode != 0 {
			return errorResult(execResultToText(result))
		}
		return textResult(fmt.Sprintf("Downloaded %s:%s to %s", in.Host, in.RemotePath, in.LocalPath))
	})

	// ssh_check
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ssh_check",
		Description: "Test SSH connectivity to a host (runs 'true' with 5s connect timeout)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in CheckInput) (*mcp.CallToolResult, any, error) {
		if in.Host == "" {
			return errorResult("host is required")
		}
		result, err := sshCheck(ctx, in.Host)
		if err != nil {
			return errorResult(fmt.Sprintf("SSH check error: %v", err))
		}
		if result.ExitCode == 0 {
			return textResult(fmt.Sprintf("OK: %s is reachable", in.Host))
		}
		return errorResult(fmt.Sprintf("FAIL: %s unreachable\n%s", in.Host, execResultToText(result)))
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
