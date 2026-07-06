package browserprofiles

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GeoInfo is the subset of ip-api.com geolocation data used for timezone detection.
type GeoInfo struct {
	Timezone string
	Country  string
	City     string
	Region   string
}

// geoIPBaseURL is the ip-api.com base URL. It is a package var so tests can point the
// lookup at an httptest server; production never reassigns it.
var geoIPBaseURL = "http://ip-api.com"

// geoIPClient is the HTTP client for ip-api.com lookups (short timeout; the API is best-effort).
var geoIPClient = &http.Client{Timeout: 10 * time.Second}

// DetectTimezoneFromIP looks up an IP's timezone via ip-api.com (free, no API key,
// 45 req/min). Returns (nil, nil) when the lookup does not succeed — mirroring the TS
// source, which swallows failures and falls back. A transport error is returned so
// callers may log it; AutoDetectTimezone ignores it.
func DetectTimezoneFromIP(ip string) (*GeoInfo, error) {
	url := fmt.Sprintf("%s/json/%s?fields=status,country,regionName,city,timezone", geoIPBaseURL, ip)
	resp, err := geoIPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var data struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		Timezone   string `json:"timezone"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data.Status != "success" {
		return nil, nil
	}
	return &GeoInfo{
		Timezone: data.Timezone,
		Country:  data.Country,
		City:     data.City,
		Region:   data.RegionName,
	}, nil
}

// AutoDetectTimezone returns the timezone for a proxy's IP, or "America/New_York" on failure.
func AutoDetectTimezone(proxy *ProxyConfig) string {
	if proxy != nil {
		if gi, _ := DetectTimezoneFromIP(proxy.Host); gi != nil {
			return gi.Timezone
		}
	}
	return "America/New_York"
}
