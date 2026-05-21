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
	Domain        string  `json:"domain"`
	DomainName    string  `json:"domain_name"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	IPAddress     string  `json:"ip_address"`
}

type DomainItem struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
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

func (c *Client) GetDomains() ([]DomainItem, error) {
	url := c.WorkerURL + "/api/domains"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Success bool          `json:"success"`
		Data    []DomainItem  `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("get domains failed")
	}

	return apiResp.Data, nil
}

func (c *Client) AddDomain(name, domain string) error {
	url := c.WorkerURL + "/api/domains"

	payload := map[string]string{"name": name, "domain": domain}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if !apiResp.Success {
		return fmt.Errorf("add domain failed: %s", apiResp.Error)
	}

	return nil
}

func (c *Client) DeleteDomain(domain string) error {
	url := c.WorkerURL + "/api/domains/" + domain

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}

	if !apiResp.Success {
		return fmt.Errorf("delete domain failed: %s", apiResp.Error)
	}

	return nil
}
