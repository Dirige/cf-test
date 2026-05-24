package geoip

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type IPInfo struct {
	IP       string `json:"ip"`
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	Province string `json:"province"`
	ISPTag   string `json:"isp_tag"`
}

var ispKeywords = map[string][]string{
	"telecom": {"ChinaTelecom", "China Telecom", "Chinanet", "AS4134", "AS4809", "电信", "telecom", "CT"},
	"unicom":  {"ChinaUnicom", "China Unicom", "AS4837", "AS9929", "AS10099", "联通", "unicom", "CU"},
	"mobile":  {"ChinaMobile", "China Mobile", "CMNET", "AS56040", "AS9808", "移动", "mobile", "CM"},
}

var ispCNNames = map[string]string{
	"telecom": "电信",
	"unicom":  "联通",
	"mobile":  "移动",
	"other":   "其他",
}

var provinceEN2CN = map[string]string{
	"Beijing":       "北京",
	"Tianjin":       "天津",
	"Hebei":         "河北",
	"Shanxi":        "山西",
	"Inner Mongolia": "内蒙古",
	"Liaoning":      "辽宁",
	"Jilin":         "吉林",
	"Heilongjiang":  "黑龙江",
	"Shanghai":      "上海",
	"Jiangsu":       "江苏",
	"Zhejiang":      "浙江",
	"Anhui":         "安徽",
	"Fujian":        "福建",
	"Jiangxi":       "江西",
	"Shandong":      "山东",
	"Henan":         "河南",
	"Hubei":         "湖北",
	"Hunan":         "湖南",
	"Guangdong":     "广东",
	"Guangxi":       "广西",
	"Hainan":        "海南",
	"Chongqing":     "重庆",
	"Sichuan":       "四川",
	"Guizhou":       "贵州",
	"Yunnan":        "云南",
	"Xizang":        "西藏",
	"Shaanxi":       "陕西",
	"Gansu":         "甘肃",
	"Qinghai":       "青海",
	"Ningxia":       "宁夏",
	"Xinjiang":      "新疆",
	"Hong Kong":     "香港",
	"Macau":         "澳门",
	"Taiwan":        "台湾",
}

func GetLocalIPInfo() (*IPInfo, error) {
	// 优先使用 ipinfo.io (免费，每月50,000次)
	info, err := getIPInfoIO()
	if err == nil && info.IP != "" {
		return info, nil
	}

	// 备用: ip.useragentinfo.com (国内准确)
	info, err = getUserAgentInfo()
	if err == nil && info.IP != "" {
		return info, nil
	}

	// 备用: ip.sb
	info, err = getIPSBInfo()
	if err == nil && info.IP != "" {
		return info, nil
	}

	// 备用: ip-api.com
	info, err = getIpApiInfo()
	if err == nil && info.IP != "" {
		return info, nil
	}

	// 备用: myip.ipip.net
	info, err = getMyIpInfo()
	if err == nil && info.IP != "" {
		return info, nil
	}

	return nil, fmt.Errorf("all geoip APIs failed")
}

// ipinfo.io API 返回格式 (免费，每月50,000次)
func getIPInfoIO() (*IPInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://ipinfo.io/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		IP      string `json:"ip"`
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
		Org     string `json:"org"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.IP == "" {
		return nil, fmt.Errorf("ipinfo.io failed")
	}

	info := &IPInfo{
		IP:       result.IP,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.Org,
		Province: normalizeProvince(result.Region),
		ISPTag:   classifyISP(result.Org),
	}

	return info, nil
}

// ip.useragentinfo.com API 返回格式 (国内准确)
func getUserAgentInfo() (*IPInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://ip.useragentinfo.com/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.IP == "" {
		return nil, fmt.Errorf("useragentinfo.com failed")
	}

	info := &IPInfo{
		IP:       result.IP,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.ISP,
		Province: normalizeProvince(result.Region),
		ISPTag:   classifyISP(result.ISP),
	}

	return info, nil
}

// ip.sb API 返回格式
func getIPSBInfo() (*IPInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://api.ip.sb/geoip")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		ASN     int    `json:"asn"`
		ASNOrg  string `json:"asn_organization"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.IP == "" {
		return nil, fmt.Errorf("ip.sb failed")
	}

	info := &IPInfo{
		IP:       result.IP,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.ISP,
		Province: normalizeProvince(result.Region),
		ISPTag:   classifyISP(result.ISP + " " + result.ASNOrg),
	}

	return info, nil
}

// ip-api.com API 返回格式
func getIpApiInfo() (*IPInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/?lang=zh-CN")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Status  string `json:"status"`
		Country string `json:"country"`
		Region  string `json:"regionName"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		Org     string `json:"org"`
		AS      string `json:"as"`
		IP      string `json:"query"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Status != "success" || result.IP == "" {
		return nil, fmt.Errorf("ip-api.com failed")
	}

	info := &IPInfo{
		IP:       result.IP,
		Country:  result.Country,
		Region:   result.Region,
		City:     result.City,
		ISP:      result.ISP,
		Province: normalizeProvince(result.Region),
		ISPTag:   classifyISP(result.ISP + " " + result.Org + " " + result.AS),
	}

	return info, nil
}

// myip.ipip.net API 返回格式
func getMyIpInfo() (*IPInfo, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://myip.ipip.net/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		IP      string `json:"ip"`
		IsIPv6  bool   `json:"is_ipv6"`
		Info    struct {
			Country  string `json:"country"`
			Province string `json:"province"`
			City     string `json:"city"`
			ISP      string `json:"isp"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.IP == "" {
		return nil, fmt.Errorf("myip.ipip.net failed")
	}

	info := &IPInfo{
		IP:       result.IP,
		Country:  result.Info.Country,
		Region:   result.Info.Province,
		City:     result.Info.City,
		ISP:      result.Info.ISP,
		Province: normalizeProvince(result.Info.Province),
		ISPTag:   classifyISP(result.Info.ISP),
	}

	return info, nil
}

func normalizeProvince(region string) string {
	region = strings.TrimSpace(region)
	region = strings.TrimSuffix(region, "省")
	region = strings.TrimSuffix(region, "市")
	region = strings.TrimSuffix(region, "壮族自治区")
	region = strings.TrimSuffix(region, "回族自治区")
	region = strings.TrimSuffix(region, "维吾尔自治区")
	region = strings.TrimSuffix(region, "特别行政区")
	region = strings.TrimSuffix(region, "自治区")
	if cn, ok := provinceEN2CN[region]; ok {
		return cn
	}
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

func GetISPDisplayName(tag string) string {
	if name, ok := ispCNNames[tag]; ok {
		return name
	}
	return "其他"
}
