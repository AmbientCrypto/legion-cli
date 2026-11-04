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

// ---- LGN-549: all-in prices and the platform fee note ----

func feeOfferFixture() api.Offer {
	o := offerFixture()
	o.RenterFeeBps = ptr(500)
	return o
}

func TestOfferListPlatformFeeNote(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{feeOfferFixture()}})
	_, errs := h.mustRun("offer", "list")
	contains(t, "list", errs, "Prices include a 5% platform fee.")
}

func TestOfferListNoFeeNoNote(t *testing.T) {
	h := newHarness(t)
	o := offerFixture()
	o.RenterFeeBps = ptr(0)
	h.reply("GET /v1/offers", 200, api.OfferPage{Items: []api.Offer{o, offerFixture()}})
	out, errs := h.mustRun("offer", "list")
	excludes(t, "list", out+errs, "platform fee")
}

func TestOfferGetPlatformFeeNote(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: feeOfferFixture(), UptimeDaily: []api.DayUptime{}})
	out, _ := h.mustRun("offer", "get", uidS(50))
	contains(t, "get", out, "Price:      $2.50 per GPU-hour", "Prices include a 5% platform fee.")
}

func TestOfferGetOwnHostFloorWording(t *testing.T) {
	h := newHarness(t)
	o := ownOfferFixture()
	o.RenterFeeBps = ptr(500)
	o.OwnUseFeeFloorPerGpuHour = ptr(money(7))
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: o, UptimeDaily: []api.DayUptime{}})
	out, _ := h.mustRun("offer", "get", uidS(50))
	contains(t, "get", out, "the greater of 5% of your listed price of $2.50 and the $0.07 minimum")
	excludes(t, "get", out, "Prices include")
}

func TestRentalCreatePreviewPlatformFeeNote(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: feeOfferFixture(), UptimeDaily: []api.DayUptime{}})
	h.rentalLifecycle("quiet-river-7k2f", 9)
	out, _ := h.mustRun(createArgs("--dry-run")...)
	contains(t, "preview", out, "Prices include a 5% platform fee.")
}

func TestRentalCreateOwnHostPreviewFloor(t *testing.T) {
	h := newHarness(t)
	h.stdFixtures()
	o := ownOfferFixture()
	o.OwnUseFeeFloorPerGpuHour = ptr(money(7))
	h.reply("GET /v1/offers/"+uidS(50), 200, api.OfferDetail{Offer: o, UptimeDaily: []api.DayUptime{}})
	h.rentalLifecycle("quiet-river-7k2f", 9)
	out, _ := h.mustRun(createArgs("--dry-run")...)
	contains(t, "preview", out, "the greater of 5% of your listed price and the $0.07 minimum, $0.13 per GPU-hour")
}

func TestRentalGetPlatformFeeNote(t *testing.T) {
	h := newHarness(t)
	r := rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning)
	r.RenterFeeBps = ptr(500)
	h.reply("GET /v1/rentals/"+uidS(9), 200, r)
	out, _ := h.mustRun("rental", "get", uidS(9))
	contains(t, "get", out, "Includes a 5% platform fee.")
	h2 := newHarness(t)
	h2.reply("GET /v1/rentals/"+uidS(9), 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning))
	out, _ = h2.mustRun("rental", "get", uidS(9))
	excludes(t, "get", out, "platform fee")
}

func TestRentalChargesShowsPlatformFee(t *testing.T) {
	h := newHarness(t)
	r := rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning)
	h.reply("GET /v1/rentals/"+uidS(9), 200, r)
	h.reply("GET /v1/rentals/"+uidS(9)+"/charges", 200, api.RentalCharges{
		RentalId: uid(9), Seconds: 3600,
		Charged: api.SignedMoney{AmountMinor: 525, Currency: "USD"}, RenterFee: &api.SignedMoney{AmountMinor: 25, Currency: "USD"},
		StorageCharged: api.SignedMoney{AmountMinor: 21, Currency: "USD"}, StorageRenterFee: &api.SignedMoney{AmountMinor: 1, Currency: "USD"},
	})
	out, _ := h.mustRun("rental", "charges", uidS(9))
	contains(t, "charges", out, "Compute:  $5.25 (includes $0.25 platform fee)", "Storage:  $0.21 (includes $0.01 platform fee)", "Total:    $5.46")
}

func TestRentalChargesNoFeeNoNote(t *testing.T) {
	h := newHarness(t)
	h.reply("GET /v1/rentals/"+uidS(9), 200, rentalFixture(9, "quiet-river-7k2f", api.RentalStateRunning))
	h.reply("GET /v1/rentals/"+uidS(9)+"/charges", 200, api.RentalCharges{
		RentalId: uid(9), Seconds: 60, Charged: api.SignedMoney{AmountMinor: 8, Currency: "USD"},
		RenterFee: &api.SignedMoney{AmountMinor: 0, Currency: "USD"}, StorageCharged: api.SignedMoney{Currency: "USD"},
	})
	out, _ := h.mustRun("rental", "charges", uidS(9))
	excludes(t, "charges", out, "platform fee")
}
