package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"cfst-server/internal/config"
	"cfst-server/internal/crypto"
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
	token, err := crypto.Decrypt(config.GetEncryptedToken())
	if err != nil {
		log.Printf("[Crypto] Warning: Failed to decrypt token: %v", err)
		token = ""
	}

	h := &Handler{
		cfg:       cfg,
		dnsClient: dns.NewClient(token, cfg.DNS.ZoneID),
		reporter:  reporter.NewClient(cfg.WorkerURL),
	}
	return h
}

func (h *Handler) SetDomains(domains []speedtest.DomainItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.domains = domains
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/speedtest", h.handleSpeedTest)
	mux.HandleFunc("/api/domains", h.handleDomains)
	mux.HandleFunc("/api/dns", h.handleDNS)
	mux.HandleFunc("/api/geoip", h.handleGeoIP)
	mux.HandleFunc("/api/status", h.handleStatus)
}

func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleSpeedTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
		return
	}

	ipInfo, err := geoip.GetLocalIPInfo()
	if err != nil {
		log.Printf("[GeoIP] Error: %v", err)
		ipInfo = &geoip.IPInfo{Province: "未知", ISPTag: "other"}
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
		Timeout: time.Duration(h.cfg.Speedtest.Timeout) * time.Second,
	}

	var results []speedtest.Result
	var bestResult *speedtest.Result

	for _, d := range domains {
		log.Printf("[SpeedTest] Testing %s (%s)...", d.Name, d.Domain)
		result := speedtest.TestDomain(ctx, d, cfg)
		results = append(results, result)

		if result.Success {
			if bestResult == nil || result.DownloadSpeed > bestResult.DownloadSpeed {
				bestResult = &result
			}

			go func(r speedtest.Result) {
				payload := reporter.ResultPayload{
					Province:      ipInfo.Province,
					ISP:           ipInfo.ISPTag,
					Domain:        r.Domain,
					DomainName:    r.DomainName,
					DownloadSpeed: r.DownloadSpeed,
					Latency:       r.Latency,
					IPAddress:     r.IP,
				}
				if err := h.reporter.ReportResult(payload); err != nil {
					log.Printf("[Reporter] Error: %v", err)
				}
			}(result)
		}
	}

	if bestResult != nil && h.cfg.Speedtest.AutoUpdateDNS && h.cfg.DNS.ZoneID != "" && h.cfg.DNS.RecordName != "" {
		go h.updateDNS(bestResult.Domain)
	}

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"province":    ipInfo.Province,
			"isp":         ipInfo.ISPTag,
			"results":     results,
			"best_result": bestResult,
		},
	}, http.StatusOK)
}

func (h *Handler) updateDNS(targetDomain string) {
	log.Printf("[DNS] Updating %s -> %s", h.cfg.DNS.RecordName, targetDomain)

	record, err := h.dnsClient.GetRecord(h.cfg.DNS.RecordName, h.cfg.DNS.RecordType)
	if err != nil {
		log.Printf("[DNS] Error getting record: %v", err)
		return
	}

	if record == nil {
		log.Printf("[DNS] Record not found, creating...")
		newRecord := dns.DNSRecord{
			Type:    h.cfg.DNS.RecordType,
			Name:    h.cfg.DNS.RecordName,
			Content: targetDomain,
			Proxied: false,
			TTL:     1,
		}
		if err := h.dnsClient.CreateRecord(newRecord); err != nil {
			log.Printf("[DNS] Error creating record: %v", err)
		}
		return
	}

	record.Content = targetDomain
	if err := h.dnsClient.UpdateRecord(record.ID, *record); err != nil {
		log.Printf("[DNS] Error updating record: %v", err)
		return
	}

	log.Printf("[DNS] Updated successfully")
}

func (h *Handler) handleDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		domains, err := h.reporter.GetDomains()
		if err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
			return
		}
		h.SetDomains(toSpeedTestDomains(domains))
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
		if err := h.reporter.AddDomain(req.Name, req.Domain); err != nil {
			jsonResponse(w, map[string]interface{}{"success": false, "error": err.Error()}, http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]interface{}{"success": true}, http.StatusOK)

	default:
		jsonResponse(w, map[string]interface{}{"success": false, "error": "Method not allowed"}, http.StatusMethodNotAllowed)
	}
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

	jsonResponse(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"domains_count": domainCount,
			"worker_url":    h.cfg.WorkerURL,
			"dns_record":    h.cfg.DNS.RecordName,
		},
	}, http.StatusOK)
}

func toSpeedTestDomains(items []reporter.DomainItem) []speedtest.DomainItem {
	result := make([]speedtest.DomainItem, len(items))
	for i, item := range items {
		result[i] = speedtest.DomainItem{
			Name:   item.Name,
			Domain: item.Domain,
		}
	}
	return result
}
