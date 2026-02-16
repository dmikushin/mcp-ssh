package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"
)

type HostInfo struct {
	Alias    string `json:"alias"`
	Hostname string `json:"hostname,omitempty"`
	User     string `json:"user,omitempty"`
	Port     string `json:"port,omitempty"`
}

func ListHosts() ([]HostInfo, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("open ssh config: %w", err)
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("parse ssh config: %w", err)
	}

	var hosts []HostInfo
	for _, host := range cfg.Hosts {
		for _, pattern := range host.Patterns {
			alias := pattern.String()
			// Skip wildcard-only patterns and negated patterns
			if alias == "*" || strings.HasPrefix(alias, "!") {
				continue
			}
			hostname, _ := cfg.Get(alias, "HostName")
			user, _ := cfg.Get(alias, "User")
			port, _ := cfg.Get(alias, "Port")

			info := HostInfo{
				Alias:    alias,
				Hostname: hostname,
				User:     user,
			}
			if port != "" && port != "22" {
				info.Port = port
			}
			hosts = append(hosts, info)
		}
	}
	return hosts, nil
}
