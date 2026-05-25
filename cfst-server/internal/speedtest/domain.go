package speedtest

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type DomainItem struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type IPResult struct {
	IP            string  `json:"ip"`
	Domain        string  `json:"domain"`
	DomainName    string  `json:"domain_name"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	Success       bool    `json:"success"`
	Error         string  `json:"error,omitempty"`
	IsIPv6        bool    `json:"is_ipv6"`
}

type TestedIP struct {
	IP            string  `json:"ip"`
	DownloadSpeed float64 `json:"download_speed"`
	Latency       float64 `json:"latency"`
	IsIPv6        bool    `json:"is_ipv6"`
}

type DomainResult struct {
	Domain        string      `json:"domain"`
	DomainName    string      `json:"domain_name"`
	IP            string      `json:"ip"`
	DownloadSpeed float64     `json:"download_speed"`
	Latency       float64     `json:"latency"`
	Success       bool        `json:"success"`
	Error         string      `json:"error,omitempty"`
	IsIPv6        bool        `json:"is_ipv6"`
	TestedIPs     []TestedIP  `json:"tested_ips,omitempty"`
}

type TestConfig struct {
	Timeout     time.Duration
	TestCount   int
	SpeedCount  int
	PingCount   int
	LatencyTopN int
	SpeedTopN   int
	CfstPath    string
	SpeedLimit  float64
	DNSServer   string
}

func DefaultTestConfig() TestConfig {
	return TestConfig{
		Timeout:     10 * time.Second,
		TestCount:   5,
		SpeedCount:  3,
		PingCount:   4,
		LatencyTopN: 10,
		SpeedTopN:   3,
		CfstPath:    "./cfst",
	}
}

type ipDomain struct {
	ip         string
	domain     string
	domainName string
	isIPv6     bool
}

type pingResult struct {
	ip      string
	latency float64
	isIPv6  bool
	domain  string
	name    string
}

// TestIPMode 方案一：IP测速模式
// 1. 从优选域名中ping出IP
// 2. 测试出前N个低延迟IP
// 3. 写入ip.txt/ipv6.txt
// 4. 调用cfst.exe测速
func TestIPMode(ctx context.Context, domains []DomainItem, cfg TestConfig) []IPResult {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.LatencyTopN == 0 {
		cfg.LatencyTopN = 10
	}
	if cfg.SpeedTopN == 0 {
		cfg.SpeedTopN = 3
	}
	if cfg.PingCount == 0 {
		cfg.PingCount = 4
	}
	if cfg.CfstPath == "" {
		cfg.CfstPath = "./cfst"
	}

	log.Printf("[方案一-IP测速] 开始测速，域名数量: %d", len(domains))

	// 第一步：从所有域名解析IP
	var allIPs []ipDomain
	for _, domain := range domains {
		ips := resolveDomain(domain.Domain, cfg.DNSServer)
		if len(ips) == 0 {
			log.Printf("[方案一-IP测速] %s (%s) DNS解析失败", domain.Name, domain.Domain)
			continue
		}
		log.Printf("[方案一-IP测速] %s (%s) 解析到 %d 个IP: %v", domain.Name, domain.Domain, len(ips), ips)
		for _, ip := range ips {
			isIPv6 := strings.Contains(ip, ":")
			allIPs = append(allIPs, ipDomain{ip: ip, domain: domain.Domain, domainName: domain.Name, isIPv6: isIPv6})
		}
	}

	// 去重
	seen := make(map[string]bool)
	var uniqueIPs []ipDomain
	for _, item := range allIPs {
		if !seen[item.ip] {
			seen[item.ip] = true
			uniqueIPs = append(uniqueIPs, item)
		}
	}

	log.Printf("[方案一-IP测速] 共 %d 个唯一IP待测延迟", len(uniqueIPs))

	// 第二步：测试延迟
	var pingResults []pingResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan bool, 50)

	for _, item := range uniqueIPs {
		wg.Add(1)
		sem <- true
		go func(item ipDomain) {
			defer wg.Done()
			defer func() { <-sem }()

			totalLatency := 0.0
			successCount := 0
			for i := 0; i < cfg.PingCount; i++ {
				latency, err := testIPLatency(item.ip, cfg.Timeout)
				if err != nil {
					continue
				}
				totalLatency += latency.Seconds() * 1000
				successCount++
			}

			if successCount > 0 {
				avgLatency := totalLatency / float64(successCount)
				mu.Lock()
				pingResults = append(pingResults, pingResult{
					ip:      item.ip,
					latency: avgLatency,
					isIPv6:  item.isIPv6,
					domain:  item.domain,
					name:    item.domainName,
				})
				mu.Unlock()
				log.Printf("[方案一-IP测速] %s (%s) IP: %s 平均延迟: %.0fms", item.domainName, item.domain, item.ip, avgLatency)
			}
		}(item)
	}
	wg.Wait()

	// 按域名分组，每个域名选取5个低延迟IP
	domainGroups := make(map[string][]pingResult)
	for _, r := range pingResults {
		domainGroups[r.domain] = append(domainGroups[r.domain], r)
	}

	perDomainN := 5
	var topIPv4, topIPv6 []pingResult
	for domain, group := range domainGroups {
		var dv4, dv6 []pingResult
		for _, r := range group {
			if r.isIPv6 {
				dv6 = append(dv6, r)
			} else {
				dv4 = append(dv4, r)
			}
		}
		sort.Slice(dv4, func(i, j int) bool { return dv4[i].latency < dv4[j].latency })
		sort.Slice(dv6, func(i, j int) bool { return dv6[i].latency < dv6[j].latency })

		n4 := perDomainN
		if len(dv4) < n4 {
			n4 = len(dv4)
		}
		topIPv4 = append(topIPv4, dv4[:n4]...)

		n6 := perDomainN
		if len(dv6) < n6 {
			n6 = len(dv6)
		}
		topIPv6 = append(topIPv6, dv6[:n6]...)

		log.Printf("[方案一-IP测速] %s 选出 %d IPv4, %d IPv6", domain, n4, n6)
	}

	log.Printf("[方案一-IP测速] 共选出 %d 个低延迟IPv4, %d 个低延迟IPv6", len(topIPv4), len(topIPv6))

	// 第三步：写入ip.txt和ipv6.txt
	workDir := filepath.Dir(cfg.CfstPath)
	if workDir == "" || workDir == "." {
		workDir = "."
	}

	ipFile := filepath.Join(workDir, "ip.txt")
	ipv6File := filepath.Join(workDir, "ipv6.txt")

	// 写入IPv4
	if len(topIPv4) > 0 {
		if err := writeIPFile(ipFile, topIPv4); err != nil {
			log.Printf("[方案一-IP测速] 写入ip.txt失败: %v", err)
		} else {
			log.Printf("[方案一-IP测速] 已写入 %d 个IPv4到 %s", len(topIPv4), ipFile)
		}
	}

	// 写入IPv6
	if len(topIPv6) > 0 {
		if err := writeIPFile(ipv6File, topIPv6); err != nil {
			log.Printf("[方案一-IP测速] 写入ipv6.txt失败: %v", err)
		} else {
			log.Printf("[方案一-IP测速] 已写入 %d 个IPv6到 %s", len(topIPv6), ipv6File)
		}
	}

	// 第四步：调用cfst.exe测速
	var results []IPResult

	// 建立IP到域名的映射
	ipToDomain := make(map[string]pingResult)
	for _, r := range topIPv4 {
		ipToDomain[r.ip] = r
	}
	for _, r := range topIPv6 {
		ipToDomain[r.ip] = r
	}

	// 测速IPv4
	if len(topIPv4) > 0 {
		log.Printf("[方案一-IP测速] 开始IPv4测速...")
		v4Results, err := runCfst(cfg.CfstPath, ipFile, cfg.SpeedTopN, 0)
		if err != nil {
			log.Printf("[方案一-IP测速] IPv4测速失败: %v", err)
		} else {
			for _, r := range v4Results {
				r.IsIPv6 = false
				if info, ok := ipToDomain[r.IP]; ok {
					r.Domain = info.domain
					r.DomainName = info.name
				}
				results = append(results, r)
			}
		}
	}

	// 测速IPv6
	if len(topIPv6) > 0 {
		log.Printf("[方案一-IP测速] 开始IPv6测速...")
		v6Results, err := runCfst(cfg.CfstPath, ipv6File, cfg.SpeedTopN, 0)
		if err != nil {
			log.Printf("[方案一-IP测速] IPv6测速失败: %v", err)
		} else {
			for _, r := range v6Results {
				r.IsIPv6 = true
				if info, ok := ipToDomain[r.IP]; ok {
					r.Domain = info.domain
					r.DomainName = info.name
				}
				results = append(results, r)
			}
		}
	}

	// 按速度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].DownloadSpeed > results[j].DownloadSpeed
	})

	// 限制结果数量：3个IPv4 + 3个IPv6
	var finalResults []IPResult
	v4Count := 0
	v6Count := 0
	for _, r := range results {
		if !r.IsIPv6 && v4Count < cfg.SpeedTopN {
			finalResults = append(finalResults, r)
			v4Count++
		} else if r.IsIPv6 && v6Count < cfg.SpeedTopN {
			finalResults = append(finalResults, r)
			v6Count++
		}
	}

	log.Printf("[方案一-IP测速] 测速完成，共 %d 个结果 (IPv4: %d, IPv6: %d)", len(finalResults), v4Count, v6Count)
	return finalResults
}

// TestDomainMode 方案二：域名测速模式（串行）
// 1. 逐个域名 DNS 解析出 IP
// 2. 直接交给 cfst 测速（cfst 自带延迟筛选 + 下载测速）
// 3. 取 cfst 输出的前3个最快IP，计算平均速度和平均延迟
func TestDomainMode(ctx context.Context, domains []DomainItem, cfg TestConfig) []DomainResult {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.SpeedCount == 0 {
		cfg.SpeedCount = 3
	}
	if cfg.CfstPath == "" {
		cfg.CfstPath = "./cfst"
	}

	log.Printf("[方案二-域名测速] 开始串行测速，域名数量: %d", len(domains))

	var results []DomainResult
	for i, domain := range domains {
		select {
		case <-ctx.Done():
			log.Printf("[方案二-域名测速] 测速被取消")
			return results
		default:
		}
		log.Printf("[方案二-域名测速] [%d/%d] 测试: %s (%s)", i+1, len(domains), domain.Name, domain.Domain)
		result := testSingleDomain(domain, cfg, i)
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Success != results[j].Success {
			return results[i].Success
		}
		return results[i].Latency < results[j].Latency
	})

	log.Printf("[方案二-域名测速] 测速完成")
	return results
}

func testSingleDomain(domain DomainItem, cfg TestConfig, index int) DomainResult {
	ips := resolveDomain(domain.Domain, cfg.DNSServer)
	if len(ips) == 0 {
		return DomainResult{
			Domain:     domain.Domain,
			DomainName: domain.Name,
			Success:    false,
			Error:      "DNS解析失败",
		}
	}

	log.Printf("[方案二-域名测速] %s (%s) 解析到 %d 个IP: %v", domain.Name, domain.Domain, len(ips), ips)

	workDir := filepath.Dir(cfg.CfstPath)
	if workDir == "" || workDir == "." {
		workDir = "."
	}

	var allTestedIPs []TestedIP

	var ipv4Only []string
	for _, ip := range ips {
		if !strings.Contains(ip, ":") {
			ipv4Only = append(ipv4Only, ip)
		}
	}
	if len(ipv4Only) == 0 {
		return DomainResult{
			Domain:     domain.Domain,
			DomainName: domain.Name,
			Success:    false,
			Error:      "无可用IPv4地址",
		}
	}

	tmpFile := filepath.Join(workDir, fmt.Sprintf("ip_%d_v4.txt", index))
	resultFile := filepath.Join(workDir, fmt.Sprintf("ip_%d_v4_result.csv", index))
	defer os.Remove(tmpFile)
	defer os.Remove(resultFile)

	if err := os.WriteFile(tmpFile, []byte(strings.Join(ipv4Only, "\n")), 0644); err != nil {
		return DomainResult{
			Domain:     domain.Domain,
			DomainName: domain.Name,
			Success:    false,
			Error:      fmt.Sprintf("写入临时文件失败: %v", err),
		}
	}

	results, err := runCfst(cfg.CfstPath, tmpFile, cfg.SpeedCount, cfg.SpeedLimit)
	if err != nil {
		return DomainResult{
			Domain:     domain.Domain,
			DomainName: domain.Name,
			Success:    false,
			Error:      fmt.Sprintf("IPv4测速失败: %v", err),
		}
	}

	for _, r := range results {
		allTestedIPs = append(allTestedIPs, TestedIP{
			IP:            r.IP,
			DownloadSpeed: r.DownloadSpeed,
			Latency:       r.Latency,
			IsIPv6:        false,
		})
	}

	if len(allTestedIPs) == 0 {
		return DomainResult{
			Domain:     domain.Domain,
			DomainName: domain.Name,
			Success:    false,
			Error:      "测速失败",
		}
	}

	sort.Slice(allTestedIPs, func(i, j int) bool {
		return allTestedIPs[i].DownloadSpeed > allTestedIPs[j].DownloadSpeed
	})

	takeN := cfg.SpeedCount
	if len(allTestedIPs) < takeN {
		takeN = len(allTestedIPs)
	}
	topIPs := allTestedIPs[:takeN]

	var totalSpeed, totalLatency float64
	for _, ip := range topIPs {
		totalSpeed += ip.DownloadSpeed
		totalLatency += ip.Latency
	}
	avgSpeed := totalSpeed / float64(len(topIPs))
	avgLatency := totalLatency / float64(len(topIPs))

	best := topIPs[0]

	log.Printf("[方案二-域名测速] %s (%s) 测试 %d 个IP，平均速度: %.2f MB/s，最佳: %s (%.2f MB/s)",
		domain.Name, domain.Domain, len(topIPs), avgSpeed, best.IP, best.DownloadSpeed)

	return DomainResult{
		Domain:        domain.Domain,
		DomainName:    domain.Name,
		IP:            best.IP,
		DownloadSpeed: avgSpeed,
		Latency:       avgLatency,
		Success:       true,
		IsIPv6:        false,
		TestedIPs:     topIPs,
	}
}

func writeIPFile(filename string, results []pingResult) error {
	var lines []string
	for _, r := range results {
		lines = append(lines, r.ip)
	}
	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0644)
}

func runCfst(cfstPath, ipFile string, speedCount int, speedLimit float64) ([]IPResult, error) {
	resultFile := strings.TrimSuffix(ipFile, filepath.Ext(ipFile)) + "_result.csv"

	args := []string{
		"-f", ipFile,
		"-dn", fmt.Sprintf("%d", speedCount),
		"-dt", "15",
		"-o", resultFile,
		"-p", fmt.Sprintf("%d", speedCount),
	}
	if speedLimit > 0 {
		args = append(args, "-sl", fmt.Sprintf("%.2f", speedLimit))
	}

	log.Printf("[CFST] 运行: %s %s", cfstPath, strings.Join(args, " "))

	cmd := exec.Command(cfstPath, args...)
	cmd.Dir = filepath.Dir(cfstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[CFST] 输出: %s", string(output))
		return nil, fmt.Errorf("cfst运行失败: %v", err)
	}

	log.Printf("[CFST] 输出: %s", string(output))

	// 解析结果
	return parseCfstResult(resultFile)
}

func parseCfstResult(filename string) ([]IPResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开结果文件失败: %v", err)
	}
	defer file.Close()

	var results []IPResult
	scanner := bufio.NewScanner(file)

	// 跳过标题行
	if scanner.Scan() {
		// 标题行
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		ip := strings.TrimSpace(parts[0])
		latency := parseFloat(strings.TrimSpace(parts[4]))
		speed := parseFloat(strings.TrimSpace(parts[5]))

		results = append(results, IPResult{
			IP:            ip,
			Latency:       latency,
			DownloadSpeed: speed,
			Success:       true,
		})
	}

	return results, nil
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func resolveDomain(domain string, dnsServer string) []string {
	var ips []string
	var err error

	if dnsServer != "" {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", dnsServer+":53")
			},
		}
		ips, err = resolver.LookupHost(context.Background(), domain)
	} else {
		ips, err = net.LookupHost(domain)
	}

	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var unique []string
	for _, ip := range ips {
		if !seen[ip] {
			seen[ip] = true
			unique = append(unique, ip)
		}
	}
	return unique
}

func testIPLatency(ip string, timeout time.Duration) (time.Duration, error) {
	addr := ip
	if strings.Contains(ip, ":") {
		addr = "[" + ip + "]"
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr+":443", timeout)
	if err != nil {
		return 0, err
	}
	latency := time.Since(start)
	conn.Close()
	return latency, nil
}
