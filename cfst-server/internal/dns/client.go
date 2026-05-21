package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	APIToken string
	ZoneID   string
}

type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

type cfResponse struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewClient(apiToken, zoneID string) *Client {
	return &Client{
		APIToken: apiToken,
		ZoneID:   zoneID,
	}
}

func (c *Client) GetRecord(recordName, recordType string) (*DNSRecord, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s&type=%s",
		c.ZoneID, recordName, recordType)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cfResp cfResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, err
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return nil, fmt.Errorf("CF API error: %s", cfResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("CF API error: unknown")
	}

	var records []DNSRecord
	if err := json.Unmarshal(cfResp.Result, &records); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	return &records[0], nil
}

func (c *Client) UpdateRecord(recordID string, record DNSRecord) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s",
		c.ZoneID, recordID)

	body, err := json.Marshal(record)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp cfResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return fmt.Errorf("CF API error: %s", cfResp.Errors[0].Message)
		}
		return fmt.Errorf("CF API error: unknown")
	}

	return nil
}

func (c *Client) CreateRecord(record DNSRecord) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", c.ZoneID)

	body, err := json.Marshal(record)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp cfResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	if !cfResp.Success {
		if len(cfResp.Errors) > 0 {
			return fmt.Errorf("CF API error: %s", cfResp.Errors[0].Message)
		}
		return fmt.Errorf("CF API error: unknown")
	}

	return nil
}
