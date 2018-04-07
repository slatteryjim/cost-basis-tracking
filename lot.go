package ledger

import (
	"fmt"
	"time"
)

type (
	// Lot represents a cost-basis lot. It has mutable fields.
	Lot struct {
		// immutable fields
		parent                 *Lot
		name                   string
		lotType                LotType
		account                Account
		currency               Currency
		originalPurchaseTime   time.Time
		originalPurchaseAmount float64 // for future reference, as we modify the amount & remaining cost basis
		originalCostBasis      float64 // for future reference, as we modify the amount & remaining cost basis

		// taxableGainsDetails is non-nil only for TaxableGains LotTypes
		taxableGainsDetails *TaxableGainsDetails

		// mutable fields
		amount            float64
		costBasis         float64
		sequenceGenerator int
	}

	// TaxableGainsDetails store the details for TaxableGain lot types.
	TaxableGainsDetails struct {
		soldAmount float64

		gains float64

		// false indicates short-term capital gains.
		isLongTerm bool
	}

	// LotType identifies what kind of lot this is.
	LotType int
)

const (
	// Asset is a basic holding of some currency.
	Asset LotType = iota
	// AssetIncome represents some sort of income, like shares acquired from a cryptocurrency fork.
	AssetIncome
	// TaxableGains represents the calculated taxable gains from a sale (or non-like exchange) of an asset.
	TaxableGains

	// OneYearForCapitalGains is used to determine shot-term vs. long-term capital gains.
	// Not sure if we need a better definition of one year?
	OneYearForCapitalGains = 24 * time.Hour * 365
)

// NewLot creates a new lot.
func NewLot(parent *Lot, name string, lotType LotType, purchaseTime time.Time, account Account, currency Currency, amount, costBasis float64) *Lot {
	return &Lot{
		parent:               parent,
		name:                 name,
		lotType:              lotType,
		originalPurchaseTime: purchaseTime,
		account:              account,
		currency:             currency,

		amount:                 amount,
		originalPurchaseAmount: amount,
		costBasis:              costBasis,
		originalCostBasis:      costBasis,
	}
}

// NewChildLot creates a child lot of the given parent, deriving the name from the parent.
func NewChildLot(parent *Lot, lotType LotType, purchaseTime time.Time, account Account, currency Currency, amount, costBasis float64) *Lot {
	return NewLot(parent, parent.nameChild(), lotType, purchaseTime, account, currency, amount, costBasis)
}

// NewTaxableGainsLot creates a TaxableGains lot, and also determines whether it's long-term or short-term.
func NewTaxableGainsLot(parent *Lot, date time.Time, soldAmount, gains float64, localCurrency Currency) *Lot {
	lot := NewChildLot(parent, TaxableGains, date, "", localCurrency, 0, 0)

	lot.taxableGainsDetails = &TaxableGainsDetails{
		soldAmount: soldAmount,
		gains:      gains,
		isLongTerm: date.Sub(parent.originalPurchaseTime) >= OneYearForCapitalGains,
	}

	return lot
}

func (lot *Lot) String() string {

	if lot.lotType == TaxableGains {
		details := lot.taxableGainsDetails
		term := "short"
		if details.isLongTerm {
			term = "long"
		}
		return fmt.Sprintf("%s\t%s Taxable Gains (%s-term) from sale of %s %0.9f: USD %f", lot.name, lot.originalPurchaseTime.Format("2006-01-02"),
			term, lot.parent.currency, details.soldAmount, details.gains)
	}

	return fmt.Sprintf("%s\t%s %s %s %0.9f\t(basis:$%f\tprice:$%f)", lot.name, lot.originalPurchaseTime.Format("2006-01-02"),
		lot.account, lot.currency, lot.amount, lot.costBasis, lot.costBasis/lot.amount)
}

func (lot *Lot) nameChild() string {
	lot.sequenceGenerator++
	return fmt.Sprintf("%s.%d", lot.name, lot.sequenceGenerator)
}

// Remove removes the given amount from the lot, and returns the costBasis represented by that.
func (lot *Lot) Remove(currency Currency, amount float64) float64 {
	if lot.currency != currency {
		panic("Lot does not contain " + currency.String() + "\n" + lot.String())
	}

	percentageToRemove := amount / lot.amount
	if RoundPlaces(percentageToRemove, 12) > 1.0 {
		panic(fmt.Sprintf("Lot has less than the needed amount %f\n%s", amount, lot))
	}

	if percentageToRemove > 0.999999999999999 {
		// let's just round up to 100%
		percentageToRemove = 1.0
	}

	// Update the existing lot
	costBasisToRemove := lot.costBasis * percentageToRemove
	lot.costBasis = lot.costBasis - costBasisToRemove
	lot.amount = lot.amount - (lot.amount * percentageToRemove)

	return costBasisToRemove
}
