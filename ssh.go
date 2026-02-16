package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 30 * time.Second

type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// rsync availability cache
var (
	localRsync     bool
	localRsyncOnce sync.Once

	remoteRsyncMu    sync.Mutex
	remoteRsyncCache = map[string]bool{}
)

func hasLocalRsync() bool {
	localRsyncOnce.Do(func() {
		_, err := exec.LookPath("rsync")
		localRsync = err == nil
	})
	return localRsync
}

func hasRemoteRsync(ctx context.Context, host string) bool {
	remoteRsyncMu.Lock()
	if v, ok := remoteRsyncCache[host]; ok {
		remoteRsyncMu.Unlock()
		return v
	}
	remoteRsyncMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", host, "command -v rsync")
	err := cmd.Run()
	has := err == nil

	remoteRsyncMu.Lock()
	remoteRsyncCache[host] = has
	remoteRsyncMu.Unlock()

	return has
}

func canUseRsync(ctx context.Context, host string) bool {
	return hasLocalRsync() && hasRemoteRsync(ctx, host)
}

func sshRun(ctx context.Context, host, command string, timeoutSec int) (*ExecResult, error) {
	timeout := defaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", host, command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("ssh exec: %w", err)
		}
	}
	return result, nil
}

func sshRunBatch(ctx context.Context, host string, commands []string, timeoutSec int) (*ExecResult, error) {
	combined := strings.Join(commands, " && ")
	return sshRun(ctx, host, combined, timeoutSec)
}

func sshCheck(ctx context.Context, host string) (*ExecResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-o", "ConnectTimeout=5", host, "true")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Stderr = err.Error()
		}
	}
	return result, nil
}

// transfer runs either rsync or scp depending on availability.
// src and dst are in the format "host:path" for remote or just "path" for local.
func transfer(ctx context.Context, host, src, dst string, timeoutSec int) (result *ExecResult, method string, err error) {
	timeout := defaultTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if canUseRsync(ctx, host) {
		method = "rsync"
		cmd = exec.CommandContext(ctx, "rsync", "-az", "--partial", "-e", "ssh", src, dst)
	} else {
		method = "scp"
		cmd = exec.CommandContext(ctx, "scp", src, dst)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	result = &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, method, fmt.Errorf("%s: %w", method, runErr)
		}
	}
	return result, method, nil
}

func fileUpload(ctx context.Context, host, localPath, remotePath string, timeoutSec int) (*ExecResult, string, error) {
	dst := fmt.Sprintf("%s:%s", host, remotePath)
	return transfer(ctx, host, localPath, dst, timeoutSec)
}

func fileDownload(ctx context.Context, host, remotePath, localPath string, timeoutSec int) (*ExecResult, string, error) {
	src := fmt.Sprintf("%s:%s", host, remotePath)
	return transfer(ctx, host, src, localPath, timeoutSec)
}
