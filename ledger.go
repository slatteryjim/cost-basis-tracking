package ledger

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/samber/lo"
)

type (
	// Ledger lets you record financial activity, tracking cost basis lots.
	Ledger struct {
		// fixed reference data
		localCurrency    Currency
		historicalPrices map[Currency]map[time.Time]float64

		// mutable data
		lots              []*Lot
		sequenceGenerator int
	}

	// Currency is a name of a currency, e.g. "USD"
	// TODO: maybe we can make this a more general concept, like "Shares", which could be a variety of things which
	// for cost-basis purposes are treated in the same manner.
	Currency string

	// Account is an account name, e.g. "Coinbase"
	Account string
)

// String returns the Currency as a string.
func (c Currency) String() string { return string(c) }

// String returns the Account as a string.
func (a Account) String() string { return string(a) }

// New creates a new Ledger.
func New(localCurrency Currency, historicalPrices map[Currency]map[time.Time]float64) *Ledger {
	return &Ledger{
		localCurrency:    localCurrency,
		historicalPrices: historicalPrices,
	}
}

// DepositNewMoney represents new investment added. There may have been transfer/deposit fees involved,
// so the costBasis may be greater than the amount ultimately deposited.
func (l *Ledger) DepositNewMoney(date time.Time, account Account, amountLocalCurrency, costBasis float64) {
	l.lots = append(l.lots, NewLot(nil, l.nameLot(), Asset, date, account, l.localCurrency, amountLocalCurrency, costBasis))
}

// Income records a new lot for income.
func (l *Ledger) Income(date time.Time, account Account, currency Currency, amount float64, cost float64, note string) {
	// TODO: record the note somewhere

	l.lots = append(l.lots, NewLot(nil, l.nameLot(), AssetIncome, date, account, currency, amount, cost))
}

// Purchase represents an exchange of
func (l *Ledger) Purchase(date time.Time, fromLotName string, toAccount Account, currency Currency, amount float64, cost float64) *Lot {
	// find the given lot
	lot := l.FindLotByName(fromLotName, l.localCurrency)

	costBasis := lot.Remove(l.localCurrency, cost)

	// create a new lot
	newLot := NewChildLot(lot, Asset, date, toAccount, currency, amount, costBasis)
	l.lots = append(l.lots, newLot)
	return newLot
}

// Purchase represents an exchange of
func (l *Ledger) Fee(date time.Time, fromLotName string, currency Currency, amount float64, applyFeeToCostBasisOfLot string, note string) {
	// Model the fee as a "sale" for localCurrency, and then record that as capital gains, and
	//  add it to some other lot's cost basis.
	feeAppliedToLot := l.findLotByName(applyFeeToCostBasisOfLot)

	valueInLocalCurrency := l.Spend(date, feeAppliedToLot.account, fromLotName, currency, amount, "fee applied: "+note)

	feeAppliedToLot.costBasis += valueInLocalCurrency
}

// Transfer removes the given amount from the existing lot, and transfers it to a new account (minus the given fee).
// The new lot has the reduced amount, but preserves the original cost basis.
func (l *Ledger) Transfer(date time.Time, fromLotName string, currency Currency, amountRemoved, feePaidFromAmount float64, toAccount Account) *Lot {
	// TODO: use `date` for something.. maybe record a separate Transactions list, associated with multiple lots
	// find the given lot
	lot := l.FindLotByName(fromLotName, currency)
	costBasis := lot.Remove(currency, amountRemoved)

	// create a new lot
	newLot := NewChildLot(lot, Asset, lot.originalPurchaseTime, toAccount, currency, amountRemoved, costBasis)
	l.lots = append(l.lots, newLot)

	// "Spend" the feePaidFromAmount.
	// We treat it as a "sale" for localCurrency, and then record it as capital gains,
	// and add the amount to the new lot's cost basis.
	valueInLocalCurrency := l.Spend(date, lot.account, newLot.name, currency, feePaidFromAmount,
		fmt.Sprintf("fee for transferring from %s to %s", lot.account, toAccount))
	newLot.costBasis += valueInLocalCurrency

	return newLot
}

// TransferMultipleLots removes amounts from the existing lots, in order, transfers them to a new account,
// spreading the given fee proportionally across the amount removed from each lot.
// Each new lot has the reduced amount, but preserves the original cost basis.
func (l *Ledger) TransferMultipleLots(date time.Time, fromLotNames []string, currency Currency, totalAmountToMove, feePaidFromAmount float64, toAccount Account) []*Lot {
	var (
		newLots []*Lot

		remainingToTransfer = totalAmountToMove
	)
	// find the given lots
	for _, name := range fromLotNames {
		lot := l.FindLotByName(name, currency)
		if remainingToTransfer <= 0 {
			panic("There's nothing left to remove from this lot: " + lot.String())
		}

		// remove either the full lot amount, or just the amount remaining to transfer
		amountToRemoveFromLot := math.Min(lot.amount, remainingToTransfer)
		remainingToTransfer -= amountToRemoveFromLot

		feePortion := feePaidFromAmount * (amountToRemoveFromLot / totalAmountToMove)

		newLot := l.Transfer(date, lot.name, currency, amountToRemoveFromLot, feePortion, toAccount)
		newLots = append(newLots, newLot)
	}

	if remainingToTransfer > 0 {
		panic(fmt.Sprintf("Insufficient funds in the lots. Remaining: %.9f", remainingToTransfer))
	} else if remainingToTransfer < 0 {
		panic(fmt.Sprintf("Too much funds transferred! Remaining: %.9f", remainingToTransfer))
	}

	return newLots
}

// TransferMultipleLotsFully removes all amounts from the existing lots, and transfers them to a new account,
// spreading the given fee proportionally across the lots.
// Each new lot has the reduced amount, but preserves the original cost basis.
func (l *Ledger) TransferMultipleLotsFully(date time.Time, fromLotNames []string, currency Currency, amountRemoved, feePaidFromAmount float64, toAccount Account) []*Lot {
	var (
		lotsTotal float64
		lots      = make([]*Lot, len(fromLotNames))
	)
	// find the given lots
	for i, name := range fromLotNames {
		lots[i] = l.FindLotByName(name, currency)
		lotsTotal += lots[i].amount
	}

	if math.Abs(lotsTotal-amountRemoved) > 0.0000000000001 {
		panic(fmt.Sprintf("Amount to remove %0.10f does not match the amount in the lots %0.10f (diff: %0.20f)", amountRemoved, lotsTotal, amountRemoved-lotsTotal))
	}

	var newLots []*Lot
	for _, lot := range lots {
		feePortion := feePaidFromAmount * (lot.amount / amountRemoved)
		newLot := l.Transfer(date, lot.name, currency, lot.amount, feePortion, toAccount)
		newLots = append(newLots, newLot)
	}
	return newLots
}

// ExchangeTaxable records an exchange of one currency for another.
// It records short or long term gains in a separate lot.
// It looks up the daily price of the sold or purchased Currency to determine the localCurrency value of the exchange.
//
// I like this representation best, rather than introducing a "Pseudo Sale (USD)" lot, sullying the ancestry:
//
//	      Starting Lot
//	        /     \
//	Purchase Lot   Taxable Gain Lot
func (l *Ledger) ExchangeTaxable(date time.Time, fromLotName string,
	soldCurrency Currency, soldAmount, feeInSoldCurrency float64, lookupSoldCurrencyPriceForTaxableGains bool,
	purchasedCurrency Currency, purchasedAmountReceived float64) *Lot {

	// TODO: feeInSoldCurrency is never used.. maybe could just make a note of it if we record a list of transactions and associated lots.

	lot := l.FindLotByName(fromLotName, soldCurrency)
	soldCostBasis := lot.Remove(soldCurrency, soldAmount)

	// create new destination lot
	var purchasedLocalCurrencyEquivalent float64
	if lookupSoldCurrencyPriceForTaxableGains {
		purchasedLocalCurrencyEquivalent = l.lookupPrice(soldCurrency, date) * soldAmount
	} else {
		purchasedLocalCurrencyEquivalent = l.lookupPrice(purchasedCurrency, date) * purchasedAmountReceived
	}
	newDestinationLot := NewChildLot(lot, Asset, date, lot.account, purchasedCurrency, purchasedAmountReceived, purchasedLocalCurrencyEquivalent)
	l.lots = append(l.lots, newDestinationLot)

	// create taxable gains lot
	l.lots = append(l.lots, NewTaxableGainsLot(lot, date, soldAmount, soldCostBasis, purchasedLocalCurrencyEquivalent, l.localCurrency,
		fmt.Sprintf("exchanging %s for %s", soldCurrency, purchasedCurrency)))

	return newDestinationLot
}

// Spend records the sale of a given currency.
// It records short or long term gains in a separate lot.
// It looks up the daily price of the sold Currency to determine the localCurrency value of the spend.
func (l *Ledger) Spend(date time.Time, feeWasFromAccount Account, fromLotName string,
	soldCurrency Currency, soldAmount float64, note string) float64 {

	// nothing to do if amount is zero
	if soldAmount == 0 {
		return 0
	}

	// exchange for localCurrency, recording the capital gains
	lot := l.FindLotByName(fromLotName, soldCurrency)
	soldCostBasis := lot.Remove(soldCurrency, soldAmount)

	// withdrawing this money from the system.. it goes into the "ether"!
	valueInLocalCurrency := l.lookupPrice(soldCurrency, date) * soldAmount

	// create taxable gains lot
	gains := valueInLocalCurrency - soldCostBasis
	if gains != 0 {
		// Terrible hack here..
		// Temporarily doing something special with the lot naming here... don't want to modify the parent lot numbering,
		// since that will muck up some existing accounting before this Spend() behavior was added.
		// Create a generic "spendCapitalGains" lot, and reuse that.
		//
		// To remove this hack:
		//   - Just create normal NewTaxableGainsLot(lot, ...), though that will modify existing numbers..
		//     So modify the existing accounting to not use the strings? Maybe each operation will return a variable with
		//     lot name that could be referenced in other places of the accounting? that way we can trace through what
		//     lots are being referred to, in the code.
		var spendCapitalGainsLot *Lot
		{
			var spendCapitalGainsLotName = lot.name + ".spendCapitalGains"
			for _, lot := range l.lots {
				if lot.name == spendCapitalGainsLotName {
					spendCapitalGainsLot = lot
				}
			}
			if spendCapitalGainsLot == nil {
				spendCapitalGainsLot = NewLot(nil, spendCapitalGainsLotName, Asset, time.Time{}, "", lot.currency, 0, 0)
				l.lots = append(l.lots, spendCapitalGainsLot)
			}
		}
		// duplicating and modifying the NewTaxableGainsLot method logic here
		//newLot := NewTaxableGainsLot(lot, date, soldAmount, gains, l.localCurrency)
		newLot := NewChildLot(spendCapitalGainsLot, TaxableGains, date, "", lot.currency, 0, 0)

		newLot.taxableGainsDetails = NewTaxableGainsDetails(
			feeWasFromAccount, lot.currency, lot.originalPurchaseTime,
			soldCostBasis, date, valueInLocalCurrency, soldAmount, note,
		)
		l.lots = append(l.lots, newLot)
	}

	return valueInLocalCurrency
}

// ExchangeTaxableMultipleLots records an exchange of one currency for another, performed across multiple lots.
// It records short or long term gains in a separate lots.
// It looks up the daily price of the sold or purchased Currency to determine the localCurrency value of the exchange.
//
// It calls ExchangeTaxable to perform the overall transfer across multiple lots.
func (l *Ledger) ExchangeTaxableMultipleLots(date time.Time, fromLotNames []string,
	soldCurrency Currency, totalAmountToSell float64, lookupSoldCurrencyPriceForTaxableGains bool,
	purchasedCurrency Currency, totalAmountToPurchase float64) {

	var (
		remainingToSell     = totalAmountToSell
		remainingToPurchase = totalAmountToPurchase
	)
	// find the given lots
	for _, name := range fromLotNames {
		lot := l.FindLotByName(name, soldCurrency)
		if remainingToSell <= 0 {
			panic("There's nothing left to sell from this lot: " + lot.String())
		}
		if remainingToPurchase <= 0 {
			panic("There's nothing left to purchase")
		}

		// remove exchange the full lot amount, or just the amount remaining to sell
		amountToSellFromLot := math.Min(lot.amount, remainingToSell)
		remainingToSell -= amountToSellFromLot

		purchasePortion := totalAmountToPurchase * (amountToSellFromLot / totalAmountToSell)
		remainingToPurchase -= purchasePortion

		l.ExchangeTaxable(date, lot.name, soldCurrency, amountToSellFromLot, 0, lookupSoldCurrencyPriceForTaxableGains, purchasedCurrency, purchasePortion)
	}

	if RoundPlaces(remainingToSell, 11) != 0 || RoundPlaces(remainingToPurchase, 11) != 0 {
		panic(fmt.Sprintf("Incorrect funds sold/purchased. Remaining: sell %.13f, purchase %.13f", remainingToSell, remainingToPurchase))
	}
}

// ExchangeNonTaxable records an exchange of one currency for another.
// It is treated as non-taxable because the lot sold was purchased on the same day, so it was bought and "sold" at the identical price.
//
// I like this representation best, rather than introducing a "Pseudo Sale (USD)" lot, sullying the ancestry:
//
//	      Starting Lot
//	        /     \
//	Purchase Lot   Taxable Gain Lot
func (l *Ledger) ExchangeNonTaxable(date time.Time, fromLotName string,
	soldCurrency Currency, soldAmount, feeInSoldCurrency float64,
	purchasedCurrency Currency, purchasedAmountReceived float64) {

	// TODO: feeInSoldCurrency is never used.. maybe could just make a note of it if we record a list of transactions and associated lots.

	lot := l.FindLotByName(fromLotName, soldCurrency)
	if !lot.originalPurchaseTime.Equal(date) && !lot.originalPurchaseTime.After(date) {
		panic(fmt.Sprintf("This is probably taxable, since the lot was sold on a different day from %v\n%s", date, lot))
	}
	soldCostBasis := lot.Remove(soldCurrency, soldAmount)

	// create new destination lot
	l.lots = append(l.lots, NewChildLot(lot, Asset, date, lot.account, purchasedCurrency, purchasedAmountReceived, soldCostBasis))
}

// MergeIdenticalLots merges identical lots into one. They all must have the same purchase price, date, and account.
func (l *Ledger) MergeIdenticalLots(purchaseDate time.Time, currency Currency, lotNames []string) *Lot {
	var (
		totalAmount, totalCostBasis, pricePerUnit float64
		account                                   Account
	)
	for i, lotName := range lotNames {
		lot := l.FindLotByName(lotName, currency)

		// verify identical date
		if lot.originalPurchaseTime != purchaseDate {
			fmt.Fprintf(os.Stderr, "All lots must have the same date %s\n%s\n", purchaseDate, lot)
			panic("Lots must have identical purchase date")
		}

		// verify identical lotPrice and account
		lotPricePerUnit := lot.costBasis / lot.amount
		if i == 0 {
			pricePerUnit = lotPricePerUnit
			account = lot.account
		} else {
			if RoundPlaces(pricePerUnit-lotPricePerUnit, 11) != 0 {
				fmt.Fprintf(os.Stderr, "All lots must have the same price %.9f\n%s\n", pricePerUnit, lot)
				panic("Lots must have identical price")
			}
			if account != lot.account {
				fmt.Fprintf(os.Stderr, "All lots must have the same account %s\n%s\n", account, lot)
				panic("Lots must have identical account")
			}
		}

		// drain the lot
		totalAmount += lot.amount
		totalCostBasis += lot.Remove(currency, lot.amount)
	}

	// create a new lot.. no parent
	newLot := NewLot(nil, l.nameLot(), Asset, purchaseDate, account, currency, totalAmount, totalCostBasis)
	l.lots = append(l.lots, newLot)
	return newLot
}

// FindLotByName finds the lot with the given name, or panics.
func (l *Ledger) FindLotByName(name string, currency Currency) *Lot {
	lot := l.findLotByName(name)
	if lot.currency != currency {
		panic("Lot does not contain " + currency.String() + "\n" + lot.String())
	}
	return lot
}

func (l *Ledger) findLotByName(name string) *Lot {
	// lot name is reused when lot is updated
	// so search through lots in reverse order, returning the first match we encounter
	for i := len(l.lots) - 1; i >= 0; i-- {
		lot := l.lots[i]
		if lot.name == name {
			return lot
		}
	}
	panic("Couldn't find lot with name: " + name)
}

func (l *Ledger) nameLot() string {
	l.sequenceGenerator++
	return strconv.Itoa(l.sequenceGenerator)
}

func (l *Ledger) lookupPrice(currency Currency, date time.Time) float64 {
	if currency == l.localCurrency {
		return 1.0
	}
	prices, ok := l.historicalPrices[currency]
	if !ok {
		panic("Missing historical prices for " + currency.String())
	}
	price, ok := prices[date]
	if !ok {
		panic("Missing " + currency.String() + " historical price for " + date.String())
	}
	return price
}

// TotalInvestment simply looks at all of the "root" localCurrency nodes, summing up their original cost basis.
// This may not be perfectly accurate, going forward:
//   - Some money might still be sitting in that localCurrency lot, but not considered "invested"
//   - Might start with non-localCurrency assets sometimes. Would need to account for them in localCurrency.
func (l *Ledger) TotalInvestment() float64 {
	var totalInvestment float64
	for _, lot := range l.lots {
		if lot.parent == nil && lot.currency == l.localCurrency {
			totalInvestment += lot.originalCostBasis
		}
	}
	return totalInvestment
}

// PrintLots will print out the full listing of the lots.
func (l *Ledger) PrintLots() string {
	b := &bytes.Buffer{}
	tw := tabwriter.NewWriter(b, 0, 4, 2, ' ', tabwriter.StripEscape)
	for _, lot := range l.lots {
		fmt.Fprintln(tw, lot)
	}
	if err := tw.Flush(); err != nil {
		panic(err.Error())
	}
	return b.String()
}

// PrintIncome prints out a report of the Income lots, and a summary.
func (l *Ledger) PrintIncome() string {
	b := &bytes.Buffer{}
	var totalIncome float64
	for _, lot := range l.lots {
		if lot.lotType == AssetIncome {
			fmt.Fprintf(b, "%s\t%s %s %s %0.9f\t(basis:%0.9f,\tprice:$%f)\n", lot.name, lot.originalPurchaseTime.Format("2006-01-02"),
				lot.account, lot.currency, lot.originalPurchaseAmount, lot.originalCostBasis, lot.originalCostBasis/lot.originalPurchaseAmount)
			totalIncome += lot.originalCostBasis
		}
	}
	fmt.Fprintf(b, "(total income: $%.2f)\n", totalIncome)
	return b.String()
}

// PrintTaxableGains prints some details..
// In the future will probably want to restrict the date ranges, e.g. to a single calendar year.
func (l *Ledger) PrintTaxableGains() string {
	b := &bytes.Buffer{}

	var (
		totalShortTerm, totalLongTerm float64

		shortTermByYear = map[int]float64{}
		longTermByYear  = map[int]float64{}
	)
	for _, lot := range l.lots {
		if lot.lotType == TaxableGains {
			var (
				details = lot.taxableGainsDetails
				gain    = details.Gains()
				year    = lot.originalPurchaseTime.Year()
			)

			if details.IsLongTerm() {
				totalLongTerm += gain
				longTermByYear[year] += gain
			} else {
				totalShortTerm += gain
				shortTermByYear[year] += gain
			}
			fmt.Fprintln(b, lot)
		}
	}

	years := lo.Union(
		lo.Keys(shortTermByYear),
		lo.Keys(longTermByYear),
	)
	sort.Ints(years)
	for _, y := range years {
		fmt.Fprintf(b, "(%d's capital gains: short-term:$%.2f long-term:$%.2f)\n", y, shortTermByYear[y], longTermByYear[y])
	}
	fmt.Fprintf(b, "(Total capital gains: short-term:$%.2f long-term:$%.2f)\n", totalShortTerm, totalLongTerm)

	return b.String()
}

// Summary represents a summary of several lots.
type Summary struct {
	Balance, Basis float64
	Lots           []*Lot
}

// PrintAccounts will print a summary of all accounts, their currencies, and the constituent lots.
func (l *Ledger) PrintAccounts() string {
	var (
		accounts   = l.AccountSummary()
		totalBasis float64
	)
	// sort names
	names := make([]Account, 0, len(accounts))
	for name := range accounts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })

	b := &bytes.Buffer{}
	for _, name := range names {
		fmt.Fprintln(b, name)
		currencyToSummary := accounts[name]

		// sort currencies
		currencies := make([]Currency, 0, len(currencyToSummary))
		for currency := range currencyToSummary {
			currencies = append(currencies, currency)
		}
		sort.Slice(currencies, func(i, j int) bool { return currencies[i] < currencies[j] })

		for _, currency := range currencies {
			summary := currencyToSummary[currency]
			fmt.Fprintf(b, "\t%s %0.9f (basis:%f\tprice:$%f)\n", currency, summary.Balance, summary.Basis, summary.Basis/summary.Balance)
			totalBasis += summary.Basis
			b := tabwriter.NewWriter(b, 0, 4, 2, ' ', tabwriter.StripEscape)
			for _, lot := range summary.Lots {
				fmt.Fprintf(b, "\xff\t\t\xff%s\n", lot)
			}
			if err := b.Flush(); err != nil {
				panic(err.Error())
			}
		}
	}
	fmt.Fprintf(b, "(Total basis: $%0.2f)\n", totalBasis)
	fmt.Fprintf(b, "(Total initial investment: $%0.2f)\n", l.TotalInvestment())
	return b.String()
}

// AccountSummary will summarize the accounts and their currencies.
func (l *Ledger) AccountSummary() map[Account]map[Currency]*Summary {
	var (
		accounts = map[Account]map[Currency]*Summary{}
	)
	for _, lot := range l.lots {
		if lot.amount > 0 && lot.lotType != TaxableGains {
			currencyToSummary, ok := accounts[lot.account]
			if !ok {
				currencyToSummary = map[Currency]*Summary{}
				accounts[lot.account] = currencyToSummary
			}
			s, ok := currencyToSummary[lot.currency]
			if !ok {
				s = &Summary{}
				currencyToSummary[lot.currency] = s
			}
			s.Balance += lot.amount
			s.Basis += lot.costBasis
			s.Lots = append(s.Lots, lot)
		}
	}
	return accounts
}

// PrintPresentValueTSV will display information on the active lots,
// including their present value based on the currentPrices provided.
// Printed as tab-separated values, suitable for pasting in to a spreadsheet.
func (l *Ledger) PrintPresentValueTSV(now time.Time, currentPrices map[Currency]float64) string {
	b := &bytes.Buffer{}
	c := csv.NewWriter(b)
	c.Comma = '\t'

	c.Write([]string{"lotName", "account", "currency", "amount", "costBasis", "origPurchaseDate",
		"daysSincePurchase", "shortOrLongTerm", "presentValue", "unrealizedGainLoss", "unrealizedGainLossPercent"})
	for _, lot := range l.lots {
		if lot.amount > 0 && lot.lotType != TaxableGains {
			daysSincePurchase := now.Sub(lot.originalPurchaseTime) / (24 * time.Hour)
			shortOrLongTerm := "longTerm"
			if daysSincePurchase < 365 {
				shortOrLongTerm = "shortTerm"
			}

			var presentValue float64
			{
				currentPrice, ok := currentPrices[lot.currency]
				if !ok {
					panic("Missing prices for currency: " + lot.currency)
				}
				presentValue = lot.amount * currentPrice
			}
			var (
				unrealizedGainLoss    = presentValue - lot.costBasis
				unrealizedGainLossPct = unrealizedGainLoss / lot.costBasis * 100
			)

			c.Write([]string{
				lot.name,
				lot.account.String(),
				lot.currency.String(),
				fmt.Sprintf("%0.9f", lot.amount),
				fmt.Sprintf("%0.2f", lot.costBasis),
				fmt.Sprintf("%v", lot.originalPurchaseTime.Format("2006-01-02")),
				fmt.Sprintf("%d", daysSincePurchase),
				shortOrLongTerm,
				fmt.Sprintf("%0.2f", presentValue),
				fmt.Sprintf("%0.2f", unrealizedGainLoss),
				fmt.Sprintf("%0.1f", unrealizedGainLossPct),
			})
		}
	}
	c.Flush()

	return b.String()
}

// PrintCapitalGainsTSV will display information on the capital gains,
// printed as tab-separated values, suitable for pasting in to a spreadsheet.
func (l *Ledger) PrintCapitalGainsTSV() string {
	b := &bytes.Buffer{}
	c := csv.NewWriter(b)
	c.Comma = '\t'

	c.Write([]string{"lotName", "year", "account", "currency", "currencyAmount", "origPurchaseDate", "costBasis", "saleDate", "proceeds", "term", "gains", "note"})
	for _, lot := range l.lots {
		if lot.lotType == TaxableGains {
			details := lot.taxableGainsDetails
			term := "short"
			if details.IsLongTerm() {
				term = "long"
			}
			c.Write([]string{
				lot.name,
				details.dateOfSale.Format("2006"),
				details.account.String(),
				details.currency.String(),
				fmt.Sprintf("%0.9f", details.soldAmount),
				details.originalPurchaseTime.Format("2006-01-02"),
				fmt.Sprintf("%0.2f", details.costBasis),
				details.dateOfSale.Format("2006-01-02"),
				fmt.Sprintf("%0.2f", details.proceeds),
				term,
				fmt.Sprintf("%0.2f", details.Gains()),
				details.note,
			})
		}
	}
	c.Flush()

	return b.String()
}

// Round rounds f to the nearest integer.
// https://gist.github.com/DavidVaini/10308388
func Round(f float64) float64 {
	if f < 0 {
		return math.Ceil(f - 0.5)
	}
	return math.Floor(f + .5)
}

// RoundPlaces rounds f to the given number of decimal places.
// https://gist.github.com/DavidVaini/10308388
func RoundPlaces(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return Round(f*shift) / shift
}
