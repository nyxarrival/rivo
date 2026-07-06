package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"rivo/internal/protocol"
)

func (c *Client) detectPublicIPs(ctx context.Context) protocol.PublicIPs {
	if !c.options.PublicIPEnabled {
		return protocol.PublicIPs{}
	}

	timeout := time.Duration(c.options.PublicIPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	checkedAt := time.Now().UnixMilli()
	result := protocol.PublicIPs{}
	if c.options.PublicIPv4Enabled {
		result.IPv4 = detectPublicIPsForNetwork(ctx, "tcp4", c.options.PublicIPv4Endpoints, timeout, true, checkedAt, c.options.Logger)
	}
	if c.options.PublicIPv6Enabled {
		result.IPv6 = detectPublicIPsForNetwork(ctx, "tcp6", c.options.PublicIPv6Endpoints, timeout, false, checkedAt, c.options.Logger)
	}
	return result
}

func detectPublicIPsForNetwork(ctx context.Context, network string, endpoints []string, timeout time.Duration, wantIPv4 bool, checkedAt int64, logger *slog.Logger) []protocol.PublicIPObservation {
	client := publicIPHTTPClient(network, timeout)
	seen := make(map[string]struct{})
	results := make([]protocol.PublicIPObservation, 0, len(endpoints))
	for _, endpoint := range uniqueEndpointURLs(endpoints) {
		ip, err := fetchPublicIP(ctx, client, endpoint, wantIPv4)
		if err != nil {
			if logger != nil {
				logger.Debug("public ip probe failed", slog.String("network", network), slog.String("endpoint", endpoint), slog.String("error", err.Error()))
			}
			continue
		}
		if _, exists := seen[ip]; exists {
			continue
		}
		seen[ip] = struct{}{}
		results = append(results, protocol.PublicIPObservation{
			IP:       ip,
			Source:   "agent_http",
			LastSeen: checkedAt,
		})
	}
	return results
}

func publicIPHTTPClient(network string, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:             nil,
			DisableKeepAlives: true,
			ForceAttemptHTTP2: false,
			DialContext: func(ctx context.Context, _ string, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
		Timeout: timeout,
	}
}

func fetchPublicIP(ctx context.Context, client *http.Client, endpoint string, wantIPv4 bool) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(raw))
	if fields := strings.Fields(ip); len(fields) > 0 {
		ip = fields[0]
	}
	return normalizePublicIP(ip, wantIPv4)
}

func normalizePublicIP(value string, wantIPv4 bool) (string, error) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	ip := net.ParseIP(value)
	if ip == nil {
		return "", fmt.Errorf("invalid ip %q", value)
	}
	if wantIPv4 {
		ipv4 := ip.To4()
		if ipv4 == nil {
			return "", fmt.Errorf("not an ipv4 address: %q", value)
		}
		if !isPublicIP(ipv4) {
			return "", fmt.Errorf("not a public ipv4 address: %q", value)
		}
		return ipv4.String(), nil
	}
	if ip.To4() != nil {
		return "", fmt.Errorf("not an ipv6 address: %q", value)
	}
	ipv6 := ip.To16()
	if ipv6 == nil || !isPublicIP(ipv6) {
		return "", fmt.Errorf("not a public ipv6 address: %q", value)
	}
	return ipv6.String(), nil
}

func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsLinkLocalUnicast() &&
		!isCGNATIPv4(ip)
}

func isCGNATIPv4(ip net.IP) bool {
	ipv4 := ip.To4()
	return ipv4 != nil && ipv4[0] == 100 && ipv4[1]&0b1100_0000 == 64
}

func uniqueEndpointURLs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
