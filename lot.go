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

		// Service:            Coinbase
		account Account
		// Asset name:          Bitcoin
		currency Currency

		// Date of purchase: 07/10/2017
		originalPurchaseTime time.Time
		// Cost basis:        $1,000.00
		costBasis float64

		// Date of sale:     01/05/2018
		dateOfSale time.Time

		// Proceeds:         $11,636.53
		proceeds float64

		soldAmount float64

		note string
	}

	// LotType identifies what kind of lot this is.
	LotType int
)

// NewTaxableGainsDetails constructs a *TaxableGainsDetails
func NewTaxableGainsDetails(account Account, currency Currency, originalPurchaseTime time.Time,
	costBasis float64, dateOfSale time.Time, proceeds float64, soldAmount float64, note string) *TaxableGainsDetails {

	return &TaxableGainsDetails{
		account:              account,
		currency:             currency,
		originalPurchaseTime: originalPurchaseTime,
		costBasis:            costBasis,
		dateOfSale:           dateOfSale,
		proceeds:             proceeds,
		soldAmount:           soldAmount,
		note:                 note,
	}
}

// Gains returns the value of the proceeds minus the cost basis.
func (d *TaxableGainsDetails) Gains() float64 {
	return d.proceeds - d.costBasis
}

// IsLongTerm returns true if the currency was held for more than one year.
// If false, the gains are to be considered short-term gains.
func (d *TaxableGainsDetails) IsLongTerm() bool {
	duration := d.dateOfSale.Sub(d.originalPurchaseTime)
	return duration >= OneYearForCapitalGains
}

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
func NewTaxableGainsLot(parent *Lot, date time.Time, soldAmount, costBasis, proceeds float64, localCurrency Currency, note string) *Lot {
	lot := NewChildLot(parent, TaxableGains, date, "", localCurrency, 0, 0)

	lot.taxableGainsDetails = NewTaxableGainsDetails(
		parent.account, parent.currency, parent.originalPurchaseTime,
		costBasis, date, proceeds, soldAmount, note)

	return lot
}

// Name returns the name of the lot.
func (lot *Lot) Name() string {
	return lot.name
}

// String returns a string describing the lot.
func (lot *Lot) String() string {

	if lot.lotType == TaxableGains {
		details := lot.taxableGainsDetails
		term := "short"
		if details.IsLongTerm() {
			term = "long"
		}
		return fmt.Sprintf("%s\t%s Taxable Gains (%s-term) from sale on %s of %s %0.9f originally purchased %s for USD %f. proceeds=USD %f, gains=USD %f, note=%s",
			lot.name, lot.originalPurchaseTime.Format("2006-01-02"),
			term, details.account, details.currency, details.soldAmount, details.originalPurchaseTime.Format("2006-01-02"),
			details.costBasis, details.proceeds, details.Gains(), details.note)
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
