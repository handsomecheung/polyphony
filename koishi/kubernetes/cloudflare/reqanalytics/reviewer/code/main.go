package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

type HostStatsRaw struct {
	Requests          int64 `json:"requests"`
	Visits            int64 `json:"visits"`
	EdgeResponseBytes int64 `json:"edgeResponseBytes"`
}

type HourlyFileRecord struct {
	Hour  string                  `json:"hour"`
	Hosts map[string]HostStatsRaw `json:"hosts"`
}

type SummaryStats struct {
	Date                    string `json:"date"`
	TotalRequests           int64  `json:"total_requests"`
	TotalBandwidth          int64  `json:"total_bandwidth"`
	TotalBandwidthFormatted string `json:"total_bandwidth_formatted"`
	TotalVisits             int64  `json:"total_visits"`
	ActiveHosts             int    `json:"active_hosts"`
	HourlyFilesCount        int    `json:"hourly_files_count"`
	LastUpdated             string `json:"last_updated"`
}

type HourlyDomainStat struct {
	Requests           int64  `json:"requests"`
	Visits             int64  `json:"visits"`
	Bandwidth          int64  `json:"bandwidth"`
	BandwidthFormatted string `json:"bandwidth_formatted"`
}

type HourlyStat struct {
	Hour               string                      `json:"hour"`
	Label              string                      `json:"label"`
	Requests           int64                       `json:"requests"`
	Visits             int64                       `json:"visits"`
	Bandwidth          int64                       `json:"bandwidth"`
	BandwidthFormatted string                      `json:"bandwidth_formatted"`
	ActiveHosts        int                         `json:"active_hosts"`
	Domains            map[string]HourlyDomainStat `json:"domains,omitempty"`
}

type HostStat struct {
	Host               string  `json:"host"`
	Domain             string  `json:"domain"`
	Requests           int64   `json:"requests"`
	RequestsPct        float64 `json:"requests_pct"`
	Visits             int64   `json:"visits"`
	Bandwidth          int64   `json:"bandwidth"`
	BandwidthFormatted string  `json:"bandwidth_formatted"`
	BandwidthPct       float64 `json:"bandwidth_pct"`
}

type DomainStat struct {
	Domain             string  `json:"domain"`
	Requests           int64   `json:"requests"`
	Visits             int64   `json:"visits"`
	Bandwidth          int64   `json:"bandwidth"`
	BandwidthFormatted string  `json:"bandwidth_formatted"`
	ActiveHosts        int     `json:"active_hosts"`
}

type DayReport struct {
	Summary  SummaryStats `json:"summary"`
	Timeline []HourlyStat `json:"timeline"`
	Hosts    []HostStat   `json:"hosts"`
	Domains  []DomainStat `json:"domains"`
}

type AnalyticsServer struct {
	dataDir     string
	mu          sync.RWMutex
	cachedAt    time.Time
	cachedDate  string
	cachedStats *DayReport
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}

func extractDomain(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if idx := strings.Index(h, ":"); idx != -1 {
		h = h[:idx]
	}
	parts := strings.Split(h, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return h
}

func findDataDirectory(configured string) string {
	candidates := []string{
		configured,
		os.Getenv("DATA_DIR"),
		"/data",
		"../../data",
		"../data",
		"./data",
	}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		hourly := filepath.Join(dir, "hourly")
		if stat, err := os.Stat(hourly); err == nil && stat.IsDir() {
			return dir
		}
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return dir
		}
	}
	if configured != "" {
		return configured
	}
	return "/data"
}

func (s *AnalyticsServer) getHourlyDir() string {
	sub := filepath.Join(s.dataDir, "hourly")
	if stat, err := os.Stat(sub); err == nil && stat.IsDir() {
		return sub
	}
	return s.dataDir
}

func (s *AnalyticsServer) resolveTargetDate(requested string) string {
	if requested != "" {
		return requested
	}

	// Default to yesterday in UTC
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	hourlyDir := s.getHourlyDir()

	matches, _ := filepath.Glob(filepath.Join(hourlyDir, yesterday+"T*.json"))
	if len(matches) > 0 {
		return yesterday
	}

	// If yesterday has no files yet, find the most recent available date
	allFiles, _ := filepath.Glob(filepath.Join(hourlyDir, "*.json"))
	datesFound := make(map[string]int)
	for _, f := range allFiles {
		base := filepath.Base(f)
		if idx := strings.Index(base, "T"); idx > 0 {
			d := base[:idx]
			datesFound[d]++
		}
	}

	var sortedDates []string
	for d := range datesFound {
		sortedDates = append(sortedDates, d)
	}
	sort.Strings(sortedDates)

	if len(sortedDates) > 0 {
		today := time.Now().UTC().Format("2006-01-02")
		for i := len(sortedDates) - 1; i >= 0; i-- {
			if sortedDates[i] < today {
				return sortedDates[i]
			}
		}
		return sortedDates[len(sortedDates)-1]
	}

	return yesterday
}

func (s *AnalyticsServer) loadYesterdayStats(requestedDate string) (*DayReport, error) {
	targetDate := s.resolveTargetDate(requestedDate)

	s.mu.RLock()
	if s.cachedStats != nil && s.cachedDate == targetDate && time.Since(s.cachedAt) < 15*time.Second {
		defer s.mu.RUnlock()
		return s.cachedStats, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedStats != nil && s.cachedDate == targetDate && time.Since(s.cachedAt) < 15*time.Second {
		return s.cachedStats, nil
	}

	hourlyDir := s.getHourlyDir()
	matches, err := filepath.Glob(filepath.Join(hourlyDir, targetDate+"T*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	report := &DayReport{
		Timeline: make([]HourlyStat, 0, 24),
	}

	var (
		totalReq   int64
		totalVis   int64
		totalBytes int64
	)

	activeHostsSet := make(map[string]struct{})
	hostAggMap := make(map[string]*HostStat)
	domainAggMap := make(map[string]*DomainStat)
	domainHostSets := make(map[string]map[string]struct{})

	// Initialize 24 hourly buckets
	hourBucketMap := make(map[string]*HourlyStat)
	for h := 0; h < 24; h++ {
		hStr := fmt.Sprintf("%02d", h)
		hourBucketMap[hStr] = &HourlyStat{
			Hour:    hStr,
			Label:   fmt.Sprintf("%02d:00", h),
			Domains: make(map[string]HourlyDomainStat),
		}
	}

	for _, filePath := range matches {
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var record HourlyFileRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}

		// Determine hour (00..23)
		var hourIndex string
		if record.Hour != "" && len(record.Hour) >= 13 {
			hourIndex = record.Hour[11:13]
		} else {
			base := filepath.Base(filePath)
			if idx := strings.Index(base, "T"); idx != -1 && len(base) >= idx+3 {
				hourIndex = base[idx+1 : idx+3]
			}
		}

		bucket := hourBucketMap[hourIndex]
		if bucket == nil {
			bucket = &HourlyStat{
				Hour:    hourIndex,
				Label:   hourIndex + ":00",
				Domains: make(map[string]HourlyDomainStat),
			}
			hourBucketMap[hourIndex] = bucket
		}

		bucket.ActiveHosts = len(record.Hosts)

		for host, stats := range record.Hosts {
			h := strings.ToLower(strings.TrimSpace(host))
			activeHostsSet[h] = struct{}{}

			totalReq += stats.Requests
			totalVis += stats.Visits
			totalBytes += stats.EdgeResponseBytes

			bucket.Requests += stats.Requests
			bucket.Visits += stats.Visits
			bucket.Bandwidth += stats.EdgeResponseBytes

			dom := extractDomain(h)

			// Record domain stats in hourly bucket
			if bucket.Domains == nil {
				bucket.Domains = make(map[string]HourlyDomainStat)
			}
			domHourly := bucket.Domains[dom]
			domHourly.Requests += stats.Requests
			domHourly.Visits += stats.Visits
			domHourly.Bandwidth += stats.EdgeResponseBytes
			bucket.Domains[dom] = domHourly

			// Host aggregation
			hs, exists := hostAggMap[h]
			if !exists {
				hs = &HostStat{
					Host:   h,
					Domain: dom,
				}
				hostAggMap[h] = hs
			}
			hs.Requests += stats.Requests
			hs.Visits += stats.Visits
			hs.Bandwidth += stats.EdgeResponseBytes

			// Domain aggregation
			ds, dExists := domainAggMap[dom]
			if !dExists {
				ds = &DomainStat{Domain: dom}
				domainAggMap[dom] = ds
			}
			ds.Requests += stats.Requests
			ds.Visits += stats.Visits
			ds.Bandwidth += stats.EdgeResponseBytes

			if domainHostSets[dom] == nil {
				domainHostSets[dom] = make(map[string]struct{})
			}
			domainHostSets[dom][h] = struct{}{}
		}
	}

	// Build sorted 24-hour timeline
	for h := 0; h < 24; h++ {
		hStr := fmt.Sprintf("%02d", h)
		b := hourBucketMap[hStr]
		b.BandwidthFormatted = formatBytes(b.Bandwidth)
		for dom, ds := range b.Domains {
			ds.BandwidthFormatted = formatBytes(ds.Bandwidth)
			b.Domains[dom] = ds
		}
		report.Timeline = append(report.Timeline, *b)
	}

	// Prepare hosts list with percentages
	floatReq := float64(totalReq)
	if floatReq == 0 {
		floatReq = 1
	}
	floatBytes := float64(totalBytes)
	if floatBytes == 0 {
		floatBytes = 1
	}

	for _, hs := range hostAggMap {
		hs.RequestsPct = (float64(hs.Requests) / floatReq) * 100
		hs.BandwidthPct = (float64(hs.Bandwidth) / floatBytes) * 100
		hs.BandwidthFormatted = formatBytes(hs.Bandwidth)
		report.Hosts = append(report.Hosts, *hs)
	}

	// Sort hosts by requests descending
	sort.Slice(report.Hosts, func(i, j int) bool {
		if report.Hosts[i].Requests == report.Hosts[j].Requests {
			return report.Hosts[i].Bandwidth > report.Hosts[j].Bandwidth
		}
		return report.Hosts[i].Requests > report.Hosts[j].Requests
	})

	// Prepare domains list
	for dom, ds := range domainAggMap {
		ds.BandwidthFormatted = formatBytes(ds.Bandwidth)
		ds.ActiveHosts = len(domainHostSets[dom])
		report.Domains = append(report.Domains, *ds)
	}
	sort.Slice(report.Domains, func(i, j int) bool {
		return report.Domains[i].Requests > report.Domains[j].Requests
	})

	report.Summary = SummaryStats{
		Date:                    targetDate,
		TotalRequests:           totalReq,
		TotalBandwidth:          totalBytes,
		TotalBandwidthFormatted: formatBytes(totalBytes),
		TotalVisits:             totalVis,
		ActiveHosts:             len(activeHostsSet),
		HourlyFilesCount:        len(matches),
		LastUpdated:             time.Now().UTC().Format(time.RFC3339),
	}

	s.cachedStats = report
	s.cachedDate = targetDate
	s.cachedAt = time.Now()
	return report, nil
}

func (s *AnalyticsServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse template: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, nil)
}

func (s *AnalyticsServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *AnalyticsServer) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	date := r.URL.Query().Get("date")
	if r.URL.Query().Get("refresh") == "1" {
		s.mu.Lock()
		s.cachedStats = nil
		s.mu.Unlock()
	}

	report, err := s.loadYesterdayStats(date)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(report)
}

func main() {
	var (
		flagDataDir string
		flagPort    string
		flagBind    string
	)

	flag.StringVar(&flagDataDir, "data-dir", "", "Path to data directory containing hourly/*.json")
	flag.StringVar(&flagPort, "port", "", "Server port (default: 8080 or env PORT)")
	flag.StringVar(&flagBind, "bind", "0.0.0.0", "Bind address")
	flag.Parse()

	port := flagPort
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	resolvedDataDir := findDataDirectory(flagDataDir)
	log.Printf("[reqanalytics-reviewer] using data directory: %s", resolvedDataDir)

	server := &AnalyticsServer{
		dataDir: resolvedDataDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/stats", server.handleStatsAPI)

	addr := fmt.Sprintf("%s:%s", flagBind, port)
	log.Printf("[reqanalytics-reviewer] server listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
