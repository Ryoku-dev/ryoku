package gateway

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// The bodies below are written from each provider's documentation of its
// account endpoint; the parsers must read them and refuse anything else.

const openRouterKeySample = `{
  "data": {
    "label": "sk-or-v1-abc...123",
    "limit": 10,
    "limit_reset": "monthly",
    "limit_remaining": 4.21,
    "include_byok_in_limit": false,
    "budgets": [],
    "effective_budget": null,
    "usage": 5.79,
    "usage_daily": 0.12,
    "usage_weekly": 1.5,
    "usage_monthly": 5.79,
    "byok_usage": 0,
    "byok_usage_daily": 0,
    "byok_usage_weekly": 0,
    "byok_usage_monthly": 0,
    "is_free_tier": false,
    "free_model_daily_requests": {"used": 36, "limit": 1000, "remaining": 964},
    "rate_limit": {"requests": 10, "interval": "10s"}
  }
}`

const openRouterUnlimitedKeySample = `{"data":{"label":"x","limit":null,"limit_reset":null,"limit_remaining":null,"usage":0.5,"is_free_tier":true,"free_model_daily_requests":{"used":3,"limit":50,"remaining":47}}}`

const openRouterCreditsSample = `{"data":{"total_credits":100.5,"total_usage":25.75}}`

const deepSeekBalanceSample = `{
  "is_available": true,
  "balance_infos": [
    {"currency": "CNY", "total_balance": "110.00", "granted_balance": "10.00", "topped_up_balance": "100.00"}
  ]
}`

const siliconFlowInfoSample = `{
  "code": 20000,
  "message": "OK",
  "status": true,
  "data": {"id": "userid", "name": "", "image": "", "email": "", "isAdmin": false,
    "balance": "0.88", "status": "normal", "introduction": "", "role": "", "chargeBalance": "88.00", "totalBalance": "88.88"}
}`

const pollinationsBalanceSample = `{"balance": 12.5}`

func TestOpenRouterKeyGivesTheFreeDailyWindowAndTheKeyCap(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	reading, err := parseOpenRouterKey([]byte(openRouterKeySample), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(reading.windows) != 1 {
		t.Fatalf("windows %+v, want the free-model daily one", reading.windows)
	}
	w := reading.windows[0]
	if w.Span != protocol.LimitSpanDay || w.Unit != protocol.LimitUnitRequests || w.Limit != 1000 || *w.Remaining != 964 || w.Source != protocol.LimitFromAccount {
		t.Fatalf("daily window %+v", w)
	}
	if !w.Reset.Equal(time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("the free count refills at UTC midnight, not %v", w.Reset)
	}
	c := reading.credits
	if c == nil || *c.Limit != 10 || *c.Remaining != 4.21 || c.Used != 5.79 || c.Currency != "USD" || c.FreeTier {
		t.Fatalf("credits %+v", c)
	}

	unlimited, err := parseOpenRouterKey([]byte(openRouterUnlimitedKeySample), now)
	if err != nil {
		t.Fatal(err)
	}
	if c := unlimited.credits; c.Limit != nil || c.Remaining != nil || c.Used != 0.5 || !c.FreeTier {
		t.Fatalf("a key with no cap reads as capped: %+v", c)
	}
	if unlimited.windows[0].Limit != 50 {
		t.Fatalf("a free-tier account has 50 free requests a day, read %d", unlimited.windows[0].Limit)
	}
	if _, err := parseOpenRouterKey([]byte(`{"error":{"code":401}}`), now); err == nil {
		t.Fatal("an error body read as a key")
	}
}

func TestOpenRouterCreditsAreTheBalance(t *testing.T) {
	c, err := parseOpenRouterCredits([]byte(openRouterCreditsSample), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if *c.Limit != 100.5 || *c.Remaining != 74.75 || c.Used != 25.75 {
		t.Fatalf("credits %+v", c)
	}
	if _, err := parseOpenRouterCredits([]byte(`{"data":{}}`), time.Now()); err == nil {
		t.Fatal("an empty answer read as a balance")
	}
}

func TestDeepSeekSiliconFlowAndPollinationsBalances(t *testing.T) {
	now := time.Now()
	deepSeek, err := parseDeepSeekBalance([]byte(deepSeekBalanceSample), now)
	if err != nil {
		t.Fatal(err)
	}
	if *deepSeek.Remaining != 110 || deepSeek.Currency != "CNY" || deepSeek.Limit != nil {
		t.Fatalf("DeepSeek %+v", deepSeek)
	}
	both := `{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"7.00"},{"currency":"USD","total_balance":"1.25"}]}`
	if c, err := parseDeepSeekBalance([]byte(both), now); err != nil || c.Currency != "USD" || *c.Remaining != 1.25 {
		t.Fatalf("with both currencies DeepSeek reads %+v, %v", c, err)
	}
	siliconFlow, err := parseSiliconFlowInfo([]byte(siliconFlowInfoSample), "USD", now)
	if err != nil {
		t.Fatal(err)
	}
	if *siliconFlow.Remaining != 88.88 || siliconFlow.Currency != "USD" {
		t.Fatalf("SiliconFlow %+v", siliconFlow)
	}
	pollinations, err := parsePollinationsBalance([]byte(pollinationsBalanceSample), now)
	if err != nil {
		t.Fatal(err)
	}
	if *pollinations.Remaining != 12.5 || pollinations.Currency != "pollen" {
		t.Fatalf("Pollinations %+v", pollinations)
	}
	for name, parse := range map[string]func([]byte) error{
		"DeepSeek":     func(b []byte) error { _, err := parseDeepSeekBalance(b, now); return err },
		"SiliconFlow":  func(b []byte) error { _, err := parseSiliconFlowInfo(b, "USD", now); return err },
		"Pollinations": func(b []byte) error { _, err := parsePollinationsBalance(b, now); return err },
	} {
		if parse([]byte(`{"message":"Authentication Fails"}`)) == nil {
			t.Fatalf("%s read an error body as a balance", name)
		}
	}
}
