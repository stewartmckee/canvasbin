package config

import (
	"testing"

	"github.com/stewartmckee/canvasbin/internal/geo"
)

func TestPricingRegions(t *testing.T) {
	c := Config{}
	uk := c.Pricing(geo.RegionUK)
	if uk.Currency != "GBP" || uk.Pro != ProAmountUK {
		t.Fatalf("uk: %+v", uk)
	}
	intl := c.Pricing(geo.RegionIntl)
	if intl.Currency != "USD" || intl.Pro != ProAmountIntl {
		t.Fatalf("intl: %+v", intl)
	}
}

func TestPricingFoghorn(t *testing.T) {
	c := Config{ProductType: "signal"}
	if c.Pricing(geo.RegionUK).Pro != FoghornAmountUK {
		t.Fatal("expected foghorn uk price")
	}
	if c.Pricing(geo.RegionIntl).Pro != FoghornAmountIntl {
		t.Fatal("expected foghorn intl price")
	}
}

func TestRegionForPriceID(t *testing.T) {
	c := Config{
		StripePricePro:         "gbp_pro",
		StripePriceProUSD:      "usd_pro",
		StripePriceTeamSeat:    "gbp_seat",
		StripePriceTeamSeatUSD: "usd_seat",
	}
	if c.RegionForPriceID("gbp_pro") != geo.RegionUK {
		t.Fatal("gbp pro")
	}
	if c.RegionForPriceID("usd_seat") != geo.RegionIntl {
		t.Fatal("usd seat")
	}
}
