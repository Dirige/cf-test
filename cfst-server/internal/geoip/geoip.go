package geoip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type IPInfo struct {
	IP       string `json:"query"`
	Country  string `json:"country"`
	Region   string `json:"regionName"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	AS       string `json:"as"`
	Province string
	ISPTag   string
}

var ispKeywords = map[string][]string{
	"telecom": {"ChinaTelecom", "China Telecom", "Chinanet", "AS4134", "AS4809"},
	"unicom":  {"ChinaUnicom", "China Unicom", "AS4837", "AS9929", "AS10099"},
	"mobile":  {"ChinaMobile", "China Mobile", "CMNET", "AS56040", "AS9808"},
}

func GetIPInfo(ip string) (*IPInfo, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=query,country,regionName,city,isp,as", ip)
	if ip == "" {
		url = "http://ip-api.com/json/?lang=zh-CN&fields=query,country,regionName,city,isp,as"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info IPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	info.Province = normalizeProvince(info.Region)
	info.ISPTag = classifyISP(info.ISP + " " + info.AS)

	return &info, nil
}

func normalizeProvince(region string) string {
	region = strings.TrimSuffix(region, "省")
	region = strings.TrimSuffix(region, "市")
	region = strings.TrimSuffix(region, "自治区")
	region = strings.TrimSuffix(region, "壮族自治区")
	region = strings.TrimSuffix(region, "回族自治区")
	region = strings.TrimSuffix(region, "维吾尔自治区")
	region = strings.TrimSuffix(region, "特别行政区")
	return region
}

func classifyISP(ispInfo string) string {
	ispInfo = strings.ToLower(ispInfo)
	for tag, keywords := range ispKeywords {
		for _, kw := range keywords {
			if strings.Contains(ispInfo, strings.ToLower(kw)) {
				return tag
			}
		}
	}
	return "other"
}

func GetLocalIPInfo() (*IPInfo, error) {
	return GetIPInfo("")
}
