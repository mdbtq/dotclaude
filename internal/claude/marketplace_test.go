package claude

import "testing"

func TestUnknownMarketplaceIsListed(t *testing.T) {
	got := loadMarketplaces(t.TempDir(), []Plugin{{Marketplace: "stray"}, {Marketplace: "stray"}})
	if len(got) != 1 || got[0].Name != "stray" || got[0].Known || got[0].Error == "" {
		t.Errorf("marketplaces: %+v, want stray once, unknown and with an error", got)
	}
}
