package config

import (
	"github.com/stewartmckee/canvasbin/internal/geo"
)

// Display amounts; keep in step with web/src/site.js applySiteConfig defaults.
const (
	ProAmountUK       = "4.99"
	ProAmountIntl     = "7.99"
	TeamBaseAmountUK  = "15"
	TeamSeatAmountUK  = "4"
	TeamBaseAmountIntl = "19"
	TeamSeatAmountIntl = "5"
	FoghornAmountUK   = "7.99"
	FoghornAmountIntl = "9.99"
)

// Pricing is what the web UI shows for a billing region.
type Pricing struct {
	Region       geo.Region `json:"region"`
	Currency     string     `json:"currency"`
	Pro          string     `json:"pro"`
	TeamBase     string     `json:"team_base"`
	TeamSeat     string     `json:"team_seat"`
	ProDisplay   string     `json:"pro_display"`
	TeamDisplay  string     `json:"team_display"` // e.g. "£15/month plus £4 per member"
}

func (c Config) Pricing(reg geo.Region) Pricing {
	proUK, proIntl := ProAmountUK, ProAmountIntl
	if c.SignalProduct() {
		proUK, proIntl = FoghornAmountUK, FoghornAmountIntl
	}
	if reg == geo.RegionUK {
		return Pricing{
			Region:      geo.RegionUK,
			Currency:    "GBP",
			Pro:         proUK,
			TeamBase:    TeamBaseAmountUK,
			TeamSeat:    TeamSeatAmountUK,
			ProDisplay:  "£" + proUK,
			TeamDisplay: "£" + TeamBaseAmountUK + "/month plus £" + TeamSeatAmountUK + " per member",
		}
	}
	return Pricing{
		Region:      geo.RegionIntl,
		Currency:    "USD",
		Pro:         proIntl,
		TeamBase:    TeamBaseAmountIntl,
		TeamSeat:    TeamSeatAmountIntl,
		ProDisplay:  "$" + proIntl,
		TeamDisplay: "$" + TeamBaseAmountIntl + "/month plus $" + TeamSeatAmountIntl + " per user",
	}
}

func (c Config) ProPriceID(reg geo.Region) string {
	if reg == geo.RegionUK || c.StripePriceProUSD == "" {
		return c.StripePricePro
	}
	return c.StripePriceProUSD
}

func (c Config) SignalPriceID(reg geo.Region) string {
	if reg == geo.RegionUK || c.StripePriceSignalUSD == "" {
		return c.StripePriceSignal
	}
	return c.StripePriceSignalUSD
}

func (c Config) TeamBasePriceID(reg geo.Region) string {
	if reg == geo.RegionUK || c.StripePriceTeamBaseUSD == "" {
		return c.StripePriceTeamBase
	}
	return c.StripePriceTeamBaseUSD
}

func (c Config) TeamSeatPriceID(reg geo.Region) string {
	if reg == geo.RegionUK || c.StripePriceTeamSeatUSD == "" {
		return c.StripePriceTeamSeat
	}
	return c.StripePriceTeamSeatUSD
}

func (c Config) RegionForPriceID(priceID string) geo.Region {
	if priceID == "" {
		return geo.RegionIntl
	}
	if priceID == c.StripePricePro || priceID == c.StripePriceSignal ||
		priceID == c.StripePriceTeamBase || priceID == c.StripePriceTeamSeat {
		return geo.RegionUK
	}
	return geo.RegionIntl
}

func (c Config) IsProPriceID(priceID string) bool {
	return priceID != "" && (priceID == c.StripePricePro || priceID == c.StripePriceProUSD)
}

func (c Config) IsSignalPriceID(priceID string) bool {
	return priceID != "" && (priceID == c.StripePriceSignal || priceID == c.StripePriceSignalUSD)
}

func (c Config) IsTeamSeatPriceID(priceID string) bool {
	return priceID != "" && (priceID == c.StripePriceTeamSeat || priceID == c.StripePriceTeamSeatUSD)
}
