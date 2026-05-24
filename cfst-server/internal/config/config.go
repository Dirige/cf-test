package config

import (
	"log"
	"os"
	"sync"

	"cfst-server/internal/crypto"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig  `yaml:"server"`
	WorkerURL string        `yaml:"worker_url"`
	DNS       DNSConfig     `yaml:"dns"`
	Speedtest SpeedConfig   `yaml:"speedtest"`
	Domains   []DomainItem  `yaml:"domains"`
	cfgPath   string        `yaml:"-"`
}

type DomainItem struct {
	Name   string `yaml:"name" json:"name"`
	Domain string `yaml:"domain" json:"domain"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DNSConfig struct {
	ZoneID     string `yaml:"zone_id"`
	RecordName string `yaml:"record_name"`
	RecordType string `yaml:"record_type"`
	APIToken   string `yaml:"api_token"`
}

type SpeedConfig struct {
	Timeout       int    `yaml:"timeout"`
	AutoUpdateDNS bool   `yaml:"auto_update_dns"`
	Schedule      string `yaml:"schedule"`
	CfstPath      string `yaml:"cfst_path"`
	DNSServer     string `yaml:"dns_server"`
}

const (
	EncryptedWorkerURL = "RPdnMqXmNMskYQovbCYwVUCJgP+76sFTFCtuoxK9lRQ="
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		DNS: DNSConfig{
			RecordType: "CNAME",
		},
		Speedtest: SpeedConfig{
			Timeout:  10,
			CfstPath: "./cfst.exe",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	workerURL, err := crypto.Decrypt(EncryptedWorkerURL)
	if err != nil {
		log.Printf("[Config] Warning: Failed to decrypt worker URL: %v", err)
	} else {
		cfg.WorkerURL = workerURL
	}

	cfg.cfgPath = path

	return cfg, nil
}

func GetEncryptedWorkerURL() string {
	return EncryptedWorkerURL
}

var saveMu sync.Mutex

func SaveToken(cfg *Config, token string) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	data, err := os.ReadFile(cfg.cfgPath)
	if err != nil {
		return err
	}

	raw := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	dnsSection, ok := raw["dns"].(map[string]interface{})
	if !ok {
		dnsSection = make(map[string]interface{})
		raw["dns"] = dnsSection
	}
	dnsSection["api_token"] = token

	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}

	return os.WriteFile(cfg.cfgPath, out, 0644)
}

func SaveDomains(cfg *Config) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	data, err := os.ReadFile(cfg.cfgPath)
	if err != nil {
		return err
	}

	raw := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	domainList := make([]map[string]string, len(cfg.Domains))
	for i, d := range cfg.Domains {
		domainList[i] = map[string]string{
			"name":   d.Name,
			"domain": d.Domain,
		}
	}
	raw["domains"] = domainList

	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}

	return os.WriteFile(cfg.cfgPath, out, 0644)
}
