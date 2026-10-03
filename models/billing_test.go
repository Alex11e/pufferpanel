package models

import "testing"

func TestBillingPlanValidation(t *testing.T) {
	tests := []struct {
		name    string
		plan    BillingPlan
		wantErr bool
	}{
		{
			name: "free plan may be inactive",
			plan: BillingPlan{Name: "Free", Currency: "huf", MaxServers: 1, Active: false},
		},
		{
			name:    "negative price is rejected",
			plan:    BillingPlan{Name: "Invalid", Currency: "HUF", OneTimePriceMinor: -1, MaxServers: 1},
			wantErr: true,
		},
		{
			name:    "invalid currency is rejected",
			plan:    BillingPlan{Name: "Invalid", Currency: "EURO", MaxServers: 1},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.plan.BeforeSave(nil)
			if (err != nil) != test.wantErr {
				t.Fatalf("BeforeSave() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && test.plan.Currency != "HUF" {
				t.Fatalf("currency was not normalized: %q", test.plan.Currency)
			}
		})
	}
}
