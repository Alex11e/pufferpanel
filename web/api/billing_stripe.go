package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pufferpanel/pufferpanel/v3/models"
)

const stripeAPIBase = "https://api.stripe.com/v1"

type stripeCheckoutSession struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	PaymentIntent  string `json:"payment_intent"`
	SubscriptionID string `json:"subscription"`
}

type stripeWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeCheckoutObject struct {
	ID                string            `json:"id"`
	ClientReferenceID string            `json:"client_reference_id"`
	PaymentIntent     string            `json:"payment_intent"`
	SubscriptionID    string            `json:"subscription"`
	PaymentStatus     string            `json:"payment_status"`
	Mode              string            `json:"mode"`
	Metadata          map[string]string `json:"metadata"`
}

func createStripeCheckout(ctx context.Context, purchase *models.BillingPurchase, plan *models.BillingPlan, publicURL string) (*stripeCheckoutSession, error) {
	secret := strings.TrimSpace(os.Getenv("PUFFER_BILLING_STRIPE_SECRET_KEY"))
	if secret == "" {
		return nil, errors.New("Stripe is not configured on the panel")
	}
	if purchase == nil || plan == nil {
		return nil, errors.New("billing purchase and plan are required")
	}
	price, ok := billingCyclePrice(plan, purchase.BillingCycle)
	if !ok || price <= 0 {
		return nil, errors.New("selected billing cycle has no paid price")
	}
	mode := "payment"
	if purchase.AutoRenew {
		if purchase.BillingCycle != models.BillingCycleMonth && purchase.BillingCycle != models.BillingCycleYear {
			return nil, errors.New("automatic renewal is only available for monthly or yearly plans")
		}
		mode = "subscription"
	}
	publicURL = strings.TrimRight(publicURL, "/")
	if publicURL == "" {
		return nil, errors.New("panel public URL is not configured")
	}
	values := url.Values{}
	values.Set("mode", mode)
	values.Set("success_url", publicURL+"/billing/complete?purchaseId="+strconv.FormatUint(uint64(purchase.ID), 10)+"&session_id={CHECKOUT_SESSION_ID}")
	values.Set("cancel_url", publicURL+"/servers/new?checkoutCancelled=1")
	values.Set("client_reference_id", strconv.FormatUint(uint64(purchase.ID), 10))
	values.Set("metadata[purchase_id]", strconv.FormatUint(uint64(purchase.ID), 10))
	values.Set("line_items[0][quantity]", "1")
	values.Set("line_items[0][price_data][currency]", strings.ToLower(plan.Currency))
	values.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(price, 10))
	values.Set("line_items[0][price_data][product_data][name]", plan.Name)
	if plan.Description != "" {
		values.Set("line_items[0][price_data][product_data][description]", plan.Description)
	}
	if mode == "subscription" {
		interval := "month"
		if purchase.BillingCycle == models.BillingCycleYear {
			interval = "year"
		}
		values.Set("line_items[0][price_data][recurring][interval]", interval)
		values.Set("subscription_data[metadata][purchase_id]", strconv.FormatUint(uint64(purchase.ID), 10))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, stripeAPIBase+"/checkout/sessions", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(secret, "")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Idempotency-Key", fmt.Sprintf("pufferpanel-purchase-%d", purchase.ID))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("Stripe checkout returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var session stripeCheckoutSession
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&session); err != nil {
		return nil, err
	}
	if session.ID == "" || session.URL == "" {
		return nil, errors.New("Stripe returned an incomplete checkout session")
	}
	return &session, nil
}

func verifyStripeWebhook(payload []byte, signatureHeader, secret string, now time.Time) bool {
	if len(payload) == 0 || secret == "" {
		return false
	}
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(signatureHeader, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 || delta(now.Unix(), unixTime) > 300 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		provided, decodeErr := hex.DecodeString(signature)
		if decodeErr == nil && hmac.Equal(provided, expected) {
			return true
		}
	}
	return false
}

func delta(left, right int64) int64 {
	if left > right {
		return left - right
	}
	return right - left
}
