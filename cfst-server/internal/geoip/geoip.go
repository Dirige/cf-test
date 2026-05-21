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
	"telecom": {"ChinaTelecom", "China Telecom", "Chinanet", "AS4134", "AS4809", "电信", "telecom"},
	"unicom":  {"ChinaUnicom", "China Unicom", "AS4837", "AS9929", "AS10099", "联通", "unicom"},
	"mobile":  {"ChinaMobile", "China Mobile", "CMNET", "AS56040", "AS9808", "移动", "mobile"},
}

type pconlineResp struct {
	IP    string `json:"ip"`
	Pro   string `json:"pro"`
	City  string `json:"city"`
	Addr  string `json:"addr"`
	Region string `json:"region"`
}

type ipApiResp struct {
	Query      string `json:"query"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
	AS         string `json:"as"`
}

func GetIPInfo(ip string) (*IPInfo, error) {
	if info, err := getPconlineInfo(ip); err == nil {
		return info, nil
	}

	return getIpApiInfo(ip)
}

func getPconlineInfo(ip string) (*IPInfo, error) {
	url := "http://whois.pconline.com.cn/ipJson.jsp?json=true"
	if ip != "" {
		url = fmt.Sprintf("http://whois.pconline.com.cn/ipJson.jsp?ip=%s&json=true", ip)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pco pconlineResp
	if err := json.NewDecoder(resp.Body).Decode(&pco); err != nil {
		return nil, err
	}

	info := &IPInfo{
		IP:      pco.IP,
		Country: "中国",
		Region:  pco.Pro,
		City:    pco.City,
		ISP:     pco.Addr,
		AS:      "",
	}

	info.Province = normalizeProvince(pco.Pro)
	info.ISPTag = classifyISP(pco.Addr)

	return info, nil
}

func getIpApiInfo(ip string) (*IPInfo, error) {
	url := "http://ip-api.com/json/?lang=zh-CN&fields=query,country,regionName,city,isp,as"
	if ip != "" {
		url = fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=query,country,regionName,city,isp,as", ip)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp ipApiResp
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	info := &IPInfo{
		IP:      apiResp.Query,
		Country: apiResp.Country,
		Region:  apiResp.RegionName,
		City:    apiResp.City,
		ISP:     apiResp.ISP,
		AS:      apiResp.AS,
	}

	info.Province = normalizeProvince(info.Region)
	info.ISPTag = classifyISP(info.ISP + " " + info.AS)

	return info, nil
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
