package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	WorkerURL string
}

type ResultPayload struct {
	Province      string  `json:"province"`
	ISP           string  `json:"isp"`
	Mode          string  `json:"mode"`
	Domain        string  `json:"domain"`
	DomainName    string  `json:"domain_name"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	IPAddress     string  `json:"ip_address"`
}

type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func NewClient(workerURL string) *Client {
	return &Client{WorkerURL: workerURL}
}

func (c *Client) ReportResult(payload ResultPayload) error {
	url := c.WorkerURL + "/api/results"

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to report result: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if !apiResp.Success {
		return fmt.Errorf("report failed: %s", apiResp.Error)
	}

	return nil
}

type BestResult struct {
	Province      string  `json:"province"`
	ISP           string  `json:"isp"`
	Mode          string  `json:"mode"`
	Domain        string  `json:"domain"`
	DomainName    string  `json:"domain_name"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	IPAddress     string  `json:"ip_address"`
	UpdatedAt     string  `json:"updated_at"`
}

func (c *Client) GetBestResults(mode string) ([]BestResult, error) {
	url := c.WorkerURL + "/api/results/best?mode=" + mode

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Success bool         `json:"success"`
		Data    []BestResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("get best results failed")
	}

	return apiResp.Data, nil
}
