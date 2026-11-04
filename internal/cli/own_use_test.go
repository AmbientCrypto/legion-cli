package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AmbientCrypto/legion-controller/api"
)

// ownOfferFixture is amber-falcon owned by the caller: listed at $2.50, own
// use at the 5% fee, 13 cents rounded half up.
func ownOfferFixture() api.Offer {
	o := offerFixture()
	o.OwnHost = ptr(true)
	o.OwnUsePricePerGpuHour = ptr(money(13))
	o.OwnUseFeeBps = ptr(500)
	return o
}

func ownRentalFixture(n int, name string, state api.RentalState) api.Rental {
	r := rentalFixture(n, name, state)
	r.OwnUse = ptr(true)
	r.OwnUseFeeBps = ptr(500)
	r.OwnUseReferencePrice = ptr(money(250))
	r.OwnUseReferenceBasis = ptr(api.OwnUseBasisOnDemand)
	r.PricePerGpuHour = money(13)
	return r
}

func TestOfferListMarksOwnHost(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{ownOfferFixture()}})
	out, _ := h.mustRun("offer", "list")
	contains(t, "list", out, "amber-falcon", "$0.13 (your host)")
	if strings.Contains(out, "$2.50") {
		t.Fatalf("shows the listed price as the price: %s", out)
	}
}

func TestOfferGetOwnHost(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: ownOfferFixture(), UptimeDaily: []api.DayUptime{}})
	out, _ := h.mustRun("offer", "get", uidS(50))
	contains(t, "get", out, "Your host:  yes", "$0.13 per GPU-hour", "5% of your listed price of $2.50", "earn nothing", "paying renter can take these GPUs")
}

func TestRentalListAndGetOwnUse(t *testing.T) {
	h := newHarness(t)
	r := ownRentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning)
	h.reply("GET /v1/rentals", 200, api.RentalPage{Items: []api.Rental{r}})
	h.reply("GET /v1/rentals/"+uidS(9), 200, r)
	out, _ := h.mustRun("rental", "list")
	contains(t, "list", out, "$0.13 (your own host)")
	out, _ = h.mustRun("rental", "get", uidS(9))
	contains(t, "get", out, "Own use:    your own host, Ambient's fee 5% of your listed price ($2.50 per GPU-hour)", "Price:      $0.13 per GPU-hour")
}

func TestRentalCreateOwnHostPreviewAndRequest(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: ownOfferFixture(), UptimeDaily: []api.DayUptime{}})
	h.rentalLifecycle("quiet-river-7k2f", 9)
	out, _ := h.mustRun(createArgs("--dry-run")...)
	contains(t, "preview", out,
		"You pay Ambient 5% of your listed price, $0.13 per GPU-hour. You earn nothing on your own use. A paying renter can take these GPUs at your listed price.",
		"$0.26 per hour")
	h.mustRun(createArgs("--yes", "--async")...)
	req := body[api.CreateRentalRequest](t, h.last("POST", "/v1/rentals"))
	if req.MaxPricePerGpuHour == nil || req.MaxPricePerGpuHour.AmountMinor != 13 {
		t.Fatalf("max price should be the fee: %+v", req.MaxPricePerGpuHour)
	}
}

func TestRentalCreateLinkedHostRefused(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.on("POST /v1/rentals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(409)
		w.Write([]byte(`{"type":"about:blank","title":"Conflict","status":409,"code":"linked_host"}`))
	})
	_, errs, code := h.run(createArgs("--yes")...)
	if code == 0 {
		t.Fatal("expected a refusal")
	}
	contains(t, "stderr", errs, "Your organization cannot rent this host.")
	for _, r := range []string{"—", "–"} {
		if strings.Contains(errs, r) {
			t.Fatalf("dash in %q", errs)
		}
	}
}
