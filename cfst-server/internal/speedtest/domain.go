package speedtest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type DomainItem struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type Result struct {
	Domain        string  `json:"domain"`
	DomainName    string  `json:"domain_name"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	IP            string  `json:"ip"`
	Success       bool    `json:"success"`
	Error         string  `json:"error,omitempty"`
}

type TestConfig struct {
	Timeout time.Duration
}

func TestDomain(ctx context.Context, item DomainItem, cfg TestConfig) Result {
	r := Result{
		Domain:     item.Domain,
		DomainName: item.Name,
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	latency, ip, err := testLatency(item.Domain, cfg.Timeout)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Latency = float64(latency.Milliseconds())
	r.IP = ip

	speed, err := testDownload(item.Domain, cfg.Timeout)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.DownloadSpeed = speed
	r.Success = true

	return r
}

func testLatency(domain string, timeout time.Duration) (time.Duration, string, error) {
	host := domain
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return 0, "", err
	}
	latency := time.Since(start)
	ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
	conn.Close()

	return latency, ip, nil
}

func testDownload(domain string, timeout time.Duration) (float64, error) {
	url := fmt.Sprintf("https://%s/", domain)

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSHandshakeTimeout: timeout,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	buf := make([]byte, 32*1024)
	totalBytes := int64(0)

	for {
		n, err := resp.Body.Read(buf)
		totalBytes += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if time.Since(start) > timeout {
			break
		}
	}

	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0, nil
	}

	speedMBps := float64(totalBytes) / elapsed / 1024 / 1024
	return speedMBps, nil
}
