package models

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	BillingCycleOnce       = "once"
	BillingCycleMonth      = "month"
	BillingCycleYear       = "year"
	BillingStatusPending   = "pending"
	BillingStatusPaid      = "paid"
	BillingStatusFailed    = "failed"
	BillingStatusCancelled = "cancelled"
)

type BillingPlan struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                string    `gorm:"not null;size:100" json:"name"`
	Description         string    `gorm:"size:2000" json:"description,omitempty"`
	Active              bool      `gorm:"not null;default:false;index" json:"active"`
	Currency            string    `gorm:"not null;size:3" json:"currency"`
	AllowOneTime        bool      `gorm:"not null;default:false" json:"allowOneTime"`
	AllowMonthly        bool      `gorm:"not null;default:false" json:"allowMonthly"`
	AllowYearly         bool      `gorm:"not null;default:false" json:"allowYearly"`
	OneTimePriceMinor   int64     `gorm:"not null;default:0" json:"oneTimePriceMinor"`
	MonthlyPriceMinor   int64     `gorm:"not null;default:0" json:"monthlyPriceMinor"`
	YearlyPriceMinor    int64     `gorm:"not null;default:0" json:"yearlyPriceMinor"`
	OneTimeDurationDays uint      `gorm:"not null;default:0" json:"oneTimeDurationDays"`
	CPUCapacityMilli    uint64    `gorm:"not null;default:0" json:"cpuCapacityMilli"`
	MemoryCapacityMB    uint64    `gorm:"not null;default:0" json:"memoryCapacityMB"`
	MaxServers          uint      `gorm:"not null;default:1" json:"maxServers"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type BillingPurchase struct {
	ID                       uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID                   uint       `gorm:"not null;index" json:"userId"`
	PlanID                   uint       `gorm:"not null;index" json:"planId"`
	NodeID                   uint       `gorm:"not null;index" json:"nodeId"`
	ServerIdentifier         *string    `gorm:"size:20;uniqueIndex" json:"serverId,omitempty"`
	Status                   string     `gorm:"not null;size:20;index" json:"status"`
	BillingCycle             string     `gorm:"not null;size:16" json:"billingCycle"`
	AutoRenew                bool       `gorm:"not null;default:false" json:"autoRenew"`
	PaymentProvider          string     `gorm:"not null;size:20" json:"paymentProvider,omitempty"`
	ProviderCheckoutID       string     `gorm:"size:255;index" json:"-"`
	ProviderPaymentID        string     `gorm:"size:255;index" json:"-"`
	ProviderSubscriptionID   string     `gorm:"size:255;index" json:"-"`
	Currency                 string     `gorm:"not null;size:3" json:"currency"`
	AmountMinor              int64      `gorm:"not null" json:"amountMinor"`
	ReservedCPUCapacityMilli uint64     `gorm:"not null;default:0" json:"-"`
	ReservedMemoryCapacityMB uint64     `gorm:"not null;default:0" json:"-"`
	ProvisionPayload         []byte     `gorm:"type:blob" json:"-"`
	ExpiresAt                *time.Time `gorm:"index" json:"expiresAt,omitempty"`
	CreatedAt                time.Time  `json:"createdAt"`
	UpdatedAt                time.Time  `json:"updatedAt"`
}

var billingCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func (p *BillingPlan) BeforeSave(*gorm.DB) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Name == "" || len(p.Name) > 100 {
		return errors.New("billing plan name must be between 1 and 100 characters")
	}
	if !billingCurrencyPattern.MatchString(p.Currency) {
		return errors.New("billing plan currency must be a three-letter ISO code")
	}
	if p.OneTimePriceMinor < 0 || p.MonthlyPriceMinor < 0 || p.YearlyPriceMinor < 0 {
		return errors.New("billing plan prices cannot be negative")
	}
	if p.MaxServers == 0 {
		return errors.New("billing plan must allow at least one server")
	}
	return nil
}
