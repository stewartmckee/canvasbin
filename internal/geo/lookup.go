package geo

import (
	"encoding/json"
	"net/http"
	"time"
)

// fetchCountryCode resolves a public IP to an ISO country code (e.g. GB, US).
// Failures return "" so callers can default to international pricing.
func fetchCountryCode(ip string) string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://ipwho.is/" + ip + "?fields=country_code,success")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var out struct {
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success {
		return ""
	}
	return countryFromHeader(out.CountryCode)
}
