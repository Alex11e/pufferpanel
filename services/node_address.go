package services

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
)

func panelNodeAddress(value string) (string, uint16, error) {
	u, err := url.Parse(value)
	if err != nil {
		return "", 0, fmt.Errorf("invalid panel URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return "", 0, fmt.Errorf("panel URL must be an HTTP(S) URL with a host and without credentials")
	}
	port := uint16(80)
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, err = parseNodePort(u.Port())
		if err != nil {
			return "", 0, err
		}
	}
	return u.Hostname(), port, nil
}

func sftpNodePort(value string) (uint16, error) {
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return 0, fmt.Errorf("invalid SFTP address: %w", err)
	}
	return parseNodePort(port)
}

func parseNodePort(value string) (uint16, error) {
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		return 0, fmt.Errorf("invalid port %q", value)
	}

	return uint16(port), nil
}

func nodeHostPort(host string, port uint16) string {
	return net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10))
}
