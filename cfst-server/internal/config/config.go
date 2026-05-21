package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig `yaml:"server"`
	WorkerURL string       `yaml:"worker_url"`
	DNS       DNSConfig    `yaml:"dns"`
	Speedtest SpeedConfig  `yaml:"speedtest"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DNSConfig struct {
	ZoneID     string `yaml:"zone_id"`
	RecordName string `yaml:"record_name"`
	RecordType string `yaml:"record_type"`
}

type SpeedConfig struct {
	Timeout       int    `yaml:"timeout"`
	AutoUpdateDNS bool   `yaml:"auto_update_dns"`
	Schedule      string `yaml:"schedule"`
}

const (
	DefaultWorkerURL    = "https://test.dirige.de5.net"
	EncryptedCFAPIToken = "KNRXW8OmvKclqqONZGva98Kdb4d/DGEMQutO/cBluuxTQtLabeZM+7a0F2oQvNQtFk4T47YIP12yeyUE4WMxhw=="
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
		WorkerURL: DefaultWorkerURL,
		DNS: DNSConfig{
			RecordType: "CNAME",
		},
		Speedtest: SpeedConfig{
			Timeout: 10,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func GetEncryptedToken() string {
	return EncryptedCFAPIToken
}
