package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"cfst-server/internal/config"
	"cfst-server/internal/dns"
	"cfst-server/internal/geoip"
	"cfst-server/internal/reporter"
	"cfst-server/internal/speedtest"
)

type Handler struct {
	cfg       *config.Config
	dnsClient *dns.Client
	reporter  *reporter.Client
	domains   []speedtest.DomainItem
	mu        sync.RWMutex
}

func NewHandler(cfg *config.Config) *Handler {
	token := cfg.DNS.APIToken
	if token == "" {
		log.Printf("[Handler] Warning: API token is empty, DNS operations will fail")
	}

	h := &Handler{
		cfg:       cfg,
		dnsClient: dns.NewClient(token, cfg.DNS.ZoneID),
		reporter:  reporter.NewClient(cfg.WorkerURL),
	}
	return h
}

func (h *Handler) LoadDomains() {
	h.mu.Lock()
	defer h.mu.Unlock()

	items := h.cfg.Domains
	if len(items) == 0 {
		log.Printf("[Domains] No domains configured in config.yaml")
		h.domains = []speedtest.DomainItem{}
		return
	}

	h.domains = make([]speedtest.DomainItem, len(items))
	for i, item := range items {
		h.domains[i] = speedtest.DomainItem{
			Name:   item.Name,
			Domain: item.Domain,
		}
	}
	log.Printf("[Domains] Loaded %d domains from config.yaml", len(items))
}

func (h *Handler) SetDomains(domains []speedtest.DomainItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.domains = domains
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/speedtest/ip", h.handleIPSpeedTest)
	mux.HandleFunc("/api/speedtest/domain", h.handleDomainSpeedTest)
	mux.HandleFunc("/api/speedtest/single", h.handleSingleDomainTest)
	mux.HandleFunc("/api/domains", h.handleDomains)
	mux.HandleFunc("/api/domains/", h.handleDomainDelete)
	mux.HandleFunc("/api/dns", h.handleDNS)
	mux.HandleFunc("/api/dns/record", h.handleDNSRecord)
	mux.HandleFunc("/api/dns/replace", h.handleDNSReplace)
	mux.HandleFunc("/api/dns/batch", h.handleDNSBatch)
	mux.HandleFunc("/api/results/best", h.handleGetBestResults)
	mux.HandleFunc("/api/token", h.handleToken)
	mux.HandleFunc("/api/geoip", h.handleGeoIP)
	mux.HandleFunc("/api/status", h.handleStatus)
}

func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleIPSpeedTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	ipInfo, err := geoip.GetLocalIPInfo()
	if err != nil {
		log.Printf("[GeoIP] Error: %v", err)
		ipInfo = &geoip.IPInfo{Province: "未知", ISPTag: "未知"}
	}

	h.mu.RLock()
	domains := make([]speedtest.DomainItem, len(h.domains))
	copy(domains, h.domains)
	h.mu.RUnlock()

	if len(domains) == 0 {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "No domains configured"}, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	cfg := speedtest.TestConfig{
		Timeout:     time.Duration(h.cfg.Speedtest.Timeout) * time.Second,
		PingCount:   4,
		LatencyTopN: 10,
		SpeedTopN:   3,
		CfstPath:    h.cfg.Speedtest.CfstPath,
		DNSServer:   h.cfg.Speedtest.DNSServer,
	}

	results := speedtest.TestIPMode(ctx, domains, cfg)

	for _, r := range results {
		if r.DownloadSpeed > 0 {
			mode := "ip"
			if r.IsIPv6 {
				mode = "ipv6"
			}
			reportPayload := reporter.ResultPayload{
				Province:      ipInfo.Province,
				ISP:           ipInfo.ISPTag,
				Mode:          mode,
				Domain:        r.Domain,
				DomainName:    r.DomainName,
				DownloadSpeed: r.DownloadSpeed,
				Latency:       r.Latency,
				IPAddress:     r.IP,
			}
			if err := h.reporter.ReportResult(reportPayload); err != nil {
				log.Printf("[Reporter] Failed to report IP result: %v", err)
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"province": ipInfo.Province,
			"isp":      ipInfo.ISPTag,
			"results":  results,
		},
	}, http.StatusOK)
}

func (h *Handler) handleDomainSpeedTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	ipInfo, err := geoip.GetLocalIPInfo()
	if err != nil {
		log.Printf("[GeoIP] Error: %v", err)
		ipInfo = &geoip.IPInfo{Province: "未知", ISPTag: "未知"}
	}

	h.mu.RLock()
	domains := make([]speedtest.DomainItem, len(h.domains))
	copy(domains, h.domains)
	h.mu.RUnlock()

	if len(domains) == 0 {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "No domains configured"}, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	cfg := speedtest.TestConfig{
		Timeout:    time.Duration(h.cfg.Speedtest.Timeout) * time.Second,
		SpeedCount: 3,
		CfstPath:   h.cfg.Speedtest.CfstPath,
		DNSServer:  h.cfg.Speedtest.DNSServer,
	}

	results := speedtest.TestDomainMode(ctx, domains, cfg)

	for _, r := range results {
		if r.Success && r.DownloadSpeed > 0 {
			mode := "domain"
			reportPayload := reporter.ResultPayload{
				Province:      ipInfo.Province,
				ISP:           ipInfo.ISPTag,
				Mode:          mode,
				Domain:        r.Domain,
				DomainName:    r.DomainName,
				DownloadSpeed: r.DownloadSpeed,
				Latency:       r.Latency,
				IPAddress:     "",
			}
			if err := h.reporter.ReportResult(reportPayload); err != nil {
				log.Printf("[Reporter] Failed to report domain result: %v", err)
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"province": ipInfo.Province,
			"isp":      ipInfo.ISPTag,
			"results":  results,
		},
	}, http.StatusOK)
}

func (h *Handler) handleDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.mu.RLock()
		domains := make([]config.DomainItem, len(h.cfg.Domains))
		copy(domains, h.cfg.Domains)
		h.mu.RUnlock()
		jsonResponse(w, map[string]interface{}{"success": true, "data": domains}, http.StatusOK)

	case http.MethodPost:
		var req struct {
			Name   string `json:"name"`
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
			return
		}
		if req.Domain == "" {
			jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain is required"}, http.StatusBadRequest)
			return
		}

		h.mu.Lock()
		for _, d := range h.cfg.Domains {
			if d.Domain == req.Domain {
				h.mu.Unlock()
				jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain already exists"}, http.StatusConflict)
				return
			}
		}
		h.cfg.Domains = append(h.cfg.Domains, config.DomainItem{Name: req.Name, Domain: req.Domain})
		if err := config.SaveDomains(h.cfg); err != nil {
			log.Printf("[Domains] Error saving config.yaml: %v", err)
		}
		h.domains = make([]speedtest.DomainItem, len(h.cfg.Domains))
		for i, item := range h.cfg.Domains {
			h.domains[i] = speedtest.DomainItem{Name: item.Name, Domain: item.Domain}
		}
		h.mu.Unlock()
		jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	default:
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleSingleDomainTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
		return
	}
	if req.Domain == "" {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain is required"}, http.StatusBadRequest)
		return
	}

	ipInfo, err := geoip.GetLocalIPInfo()
	if err != nil {
		log.Printf("[GeoIP] Error: %v", err)
		ipInfo = &geoip.IPInfo{Province: "未知", ISPTag: "未知"}
	}

	domainItem := speedtest.DomainItem{
		Name:   req.Name,
		Domain: req.Domain,
	}

	ctx := context.Background()
	cfg := speedtest.TestConfig{
		Timeout:    time.Duration(h.cfg.Speedtest.Timeout) * time.Second,
		SpeedCount: 3,
		CfstPath:   h.cfg.Speedtest.CfstPath,
		DNSServer:  h.cfg.Speedtest.DNSServer,
	}

	results := speedtest.TestDomainMode(ctx, []speedtest.DomainItem{domainItem}, cfg)

	for _, r := range results {
		if r.Success && r.DownloadSpeed > 0 {
			reportPayload := reporter.ResultPayload{
				Province:      ipInfo.Province,
				ISP:           ipInfo.ISPTag,
				Mode:          "domain",
				Domain:        r.Domain,
				DomainName:    r.DomainName,
				DownloadSpeed: r.DownloadSpeed,
				Latency:       r.Latency,
				IPAddress:     "",
			}
			if err := h.reporter.ReportResult(reportPayload); err != nil {
				log.Printf("[Reporter] Failed to report single domain result: %v", err)
			}
		}
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"province": ipInfo.Province,
			"isp":      ipInfo.ISPTag,
			"results":  results,
		},
	}, http.StatusOK)
}

func (h *Handler) handleDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	record, err := h.dnsClient.GetRecord(h.cfg.DNS.RecordName, h.cfg.DNS.RecordType)
	if err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"record_name": h.cfg.DNS.RecordName,
			"record_type": h.cfg.DNS.RecordType,
			"record":      record,
		},
	}, http.StatusOK)
}

func (h *Handler) handleDNSRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
		return
	}

	record, err := h.dnsClient.GetRecord(req.Name, req.Type)
	if err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	if record == nil {
		newRecord := dns.DNSRecord{
			Type:    req.Type,
			Name:    req.Name,
			Content: req.Content,
			Proxied: false,
			TTL:     1,
		}
		if err := h.dnsClient.CreateRecord(newRecord); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
			return
		}
	} else {
		record.Content = req.Content
		if err := h.dnsClient.UpdateRecord(record.ID, *record); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
			return
		}
	}

	jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)
}

func (h *Handler) handleDNSReplace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
		return
	}

	records, err := h.dnsClient.ListRecords(req.Name)
	if err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	for _, record := range records {
		if record.Name == req.Name {
			h.dnsClient.DeleteRecord(record.ID)
		}
	}

	newRecord := dns.DNSRecord{
		Type:    req.Type,
		Name:    req.Name,
		Content: req.Content,
		Proxied: false,
		TTL:     1,
	}
	if err := h.dnsClient.CreateRecord(newRecord); err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)
}

func (h *Handler) handleDNSBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Content []string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
		return
	}

	if len(req.Content) == 0 {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "No content provided"}, http.StatusBadRequest)
		return
	}

	var createdRecords []dns.DNSRecord
	var errors []string

	for _, content := range req.Content {
		recordType := req.Type
		if recordType == "" {
			if strings.Contains(content, ":") {
				recordType = "AAAA"
			} else {
				recordType = "A"
			}
		}
		newRecord := dns.DNSRecord{
			Type:    recordType,
			Name:    req.Name,
			Content: content,
			Proxied: false,
			TTL:     1,
		}
		if err := h.dnsClient.CreateRecord(newRecord); err != nil {
			errors = append(errors, err.Error())
			log.Printf("[DNS Batch] Failed to create record %s -> %s: %v", req.Name, content, err)
		} else {
			createdRecords = append(createdRecords, newRecord)
			log.Printf("[DNS Batch] Created record %s -> %s (%s)", req.Name, content, req.Type)
		}
	}

	if len(errors) > 0 {
		jsonResponse(w, map[string]interface{}{
			"success": false,
			"error":   strings.Join(errors, "; "),
			"data": map[string]interface{}{
				"created": len(createdRecords),
				"failed":  len(errors),
			},
		}, http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"created": len(createdRecords),
			"records": createdRecords,
		},
	}, http.StatusOK)
}

func (h *Handler) handleGeoIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	ipInfo, err := geoip.GetLocalIPInfo()
	if err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    ipInfo,
	}, http.StatusOK)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	domainCount := len(h.domains)
	h.mu.RUnlock()

	maskedURL := ""
	if len(h.cfg.WorkerURL) > 12 {
		maskedURL = h.cfg.WorkerURL[:8] + "***" + h.cfg.WorkerURL[len(h.cfg.WorkerURL)-4:]
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"domains_count": domainCount,
			"worker_url":    maskedURL,
			"dns_record":    h.cfg.DNS.RecordName,
		},
	}, http.StatusOK)
}

func (h *Handler) handleGetBestResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "ip"
	}

	results, err := h.reporter.GetBestResults(mode)
	if err != nil {
		jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data":    results,
	}, http.StatusOK)
}

func (h *Handler) handleToken(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		token := h.cfg.DNS.APIToken
		masked := ""
		if len(token) > 8 {
			masked = token[:4] + "****" + token[len(token)-4:]
		} else if len(token) > 0 {
			masked = "****"
		}
		jsonResponse(w, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"configured": token != "",
				"masked":     masked,
			},
		}, http.StatusOK)

	case http.MethodPost:
		var req struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
			return
		}
		if req.Token == "" {
			jsonResponse(w, map[string]interface{}{"success": false, "error": "Token cannot be empty"}, http.StatusBadRequest)
			return
		}

		h.cfg.DNS.APIToken = req.Token
		h.dnsClient = dns.NewClient(req.Token, h.cfg.DNS.ZoneID)

		if err := config.SaveToken(h.cfg, req.Token); err != nil {
			log.Printf("[Token] Warning: Failed to persist token to config.yaml: %v", err)
		}

		jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	default:
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimPrefix(r.URL.Path, "/api/domains/")
	if domain == "" {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain is required"}, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		h.mu.Lock()
		found := false
		newDomains := make([]config.DomainItem, 0, len(h.cfg.Domains))
		for _, d := range h.cfg.Domains {
			if d.Domain == domain {
				found = true
			} else {
				newDomains = append(newDomains, d)
			}
		}
		if !found {
			h.mu.Unlock()
			jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain not found"}, http.StatusNotFound)
			return
		}
		h.cfg.Domains = newDomains
		if err := config.SaveDomains(h.cfg); err != nil {
			log.Printf("[Domains] Error saving config.yaml: %v", err)
		}
		h.domains = make([]speedtest.DomainItem, len(h.cfg.Domains))
		for i, item := range h.cfg.Domains {
			h.domains[i] = speedtest.DomainItem{Name: item.Name, Domain: item.Domain}
		}
		h.mu.Unlock()
		jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	case http.MethodPut:
		var req struct {
			Name      string `json:"name"`
			NewDomain string `json:"new_domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusBadRequest)
			return
		}

		h.mu.Lock()
		found := false
		for i, d := range h.cfg.Domains {
			if d.Domain == domain {
				found = true
				if req.Name != "" {
					h.cfg.Domains[i].Name = req.Name
				}
				if req.NewDomain != "" && req.NewDomain != domain {
					for _, other := range h.cfg.Domains {
						if other.Domain == req.NewDomain {
							h.mu.Unlock()
							jsonResponse(w, map[string]interface{}{"success": false, "error": "Target domain already exists"}, http.StatusConflict)
							return
						}
					}
					h.cfg.Domains[i].Domain = req.NewDomain
				}
				break
			}
		}
		if !found {
			h.mu.Unlock()
			jsonResponse(w, map[string]interface{}{"success": false, "error": "Domain not found"}, http.StatusNotFound)
			return
		}

		if err := config.SaveDomains(h.cfg); err != nil {
			log.Printf("[Domains] Error saving config.yaml: %v", err)
		}
		h.domains = make([]speedtest.DomainItem, len(h.cfg.Domains))
		for i, item := range h.cfg.Domains {
			h.domains[i] = speedtest.DomainItem{Name: item.Name, Domain: item.Domain}
		}
		h.mu.Unlock()
		jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	default:
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}


