package ledger_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/slatteryjim/cost-basis-tracking"
)

const (
	USD = ledger.Currency("USD")

	// crytocurrencies
	BCH  = ledger.Currency("BCH")
	BTC  = ledger.Currency("BTC")
	BTG  = ledger.Currency("BTG")
	DASH = ledger.Currency("DASH")
	ETH  = ledger.Currency("ETH")

	// accounts
	Bitfinex = ledger.Account("Bitfinex")
	Coinbase = ledger.Account("Coinbase")
)

var (
	historicalPrices = map[ledger.Currency]map[time.Time]float64{
		BTC: {
			d("2017-11-01"): 6767.31,
			d("2017-11-02"): 6960.07,
		},
		ETH: {
			d("2017-11-01"): 291.69,
		},
	}
)

func TestSimpleScenario(t *testing.T) {
	g := NewGomegaWithT(t)

	b := &bytes.Buffer{}

	// Create a ledger, add some sample activity, then print various reports.
	//
	// We'll see income and capital gains calculated, and a summary
	// of all our current positions organized by account and currency
	// and broken-down by cost basis lots.
	simpleScenario(b)

	g.Expect(b.String()).To(BeEquivalentTo(
		`=== Cost Basis Lots: ===
1        2017-04-06 Bitfinex USD 0.000000000  (basis:$0.000000     price:$NaN)
1.1      2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689    price:$1292.025388)
1.1.1    2017-04-06 Coinbase BTC 0.799000000  (basis:$1039.095595  price:$1300.495113)
1.1.1.1  2017-11-01 Coinbase USD 0.000000000  (basis:$0.000000     price:$NaN)
1.1.1.2  2017-11-01 Taxable Gains (short-term) from sale of BTC 0.001000000: USD 5.475285

=== Capital Gains: ===
1.1.1.2	2017-11-01 Taxable Gains (short-term) from sale of BTC 0.001000000: USD 5.475285
(2017's capital gains: short-term:$5.48 long-term:$0.00)
(Total capital gains: short-term:$5.48 long-term:$0.00)

=== Account balances (and their lots): ===
Bitfinex
	BTC 0.039766780 (basis:51.379689	price:$1292.025388)
		1.1  2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689  price:$1292.025388)
Coinbase
	BTC 0.799000000 (basis:1039.095595	price:$1300.495113)
		1.1.1  2017-04-06 Coinbase BTC 0.799000000  (basis:$1039.095595  price:$1300.495113)
(Total basis: $1090.48)
(Total initial investment: $1085.00)

=== Present Value, Tab-Separated (to copy into spreadsheet): ===
lotName	account	currency	amount	costBasis	origPurchaseDate	daysSincePurchase	shortOrLongTerm	presentValue	unrealizedGainLoss	unrealizedGainLossPercent
1.1	Bitfinex	BTC	0.039766780	51.38	2017-04-06	626	longTerm	160.22	108.84	211.8
1.1.1	Coinbase	BTC	0.799000000	1039.10	2017-04-06	626	longTerm	3219.08	2179.99	209.8

`))
}

func TestLargerScenario(t *testing.T) {
	g := NewGomegaWithT(t)

	b := &bytes.Buffer{}

	// Create a ledger, add some sample activity, then print various reports.
	//
	// We'll see income and capital gains calculated, and a summary
	// of all our current positions organized by account and currency
	// and broken-down by cost basis lots.
	largerScenario(b)

	g.Expect(b.String()).To(BeEquivalentTo(
		`=== Lots: ===
1        2017-04-06 Bitfinex USD 0.000000000   (basis:$0.000000    price:$NaN)
1.1      2017-04-06 Bitfinex BTC 0.000000000   (basis:$0.000000    price:$NaN)
1.2      2017-04-06 Bitfinex ETH 0.000000000   (basis:$0.000000    price:$NaN)
1.3      2017-04-06 Bitfinex DASH 0.000000000  (basis:$0.000000    price:$NaN)
2        2017-08-01 Bitfinex BCH 0.000000000   (basis:$0.000000    price:$NaN)
3        2017-10-23 Bitfinex BTG 0.000000000   (basis:$0.000000    price:$NaN)
1.1.1    2017-04-06 Coinbase BTC 0.418873380   (basis:$575.430314  price:$1373.757182)
1.1.1.1  2017-11-01 Coinbase USD 0.000000000   (basis:$0.000000    price:$NaN)
1.1.1.2  2017-11-01 Taxable Gains (short-term) from sale of BTC 0.001000000: USD 5.409741
1.2.1    2017-04-06 Coinbase ETH 9.190000000  (basis:$260.691223  price:$28.366836)
1.2.1.1  2017-11-01 Coinbase USD 0.000000000  (basis:$0.000000    price:$NaN)
1.2.1.2  2017-11-01 Taxable Gains (short-term) from sale of ETH 0.010000000: USD 2.636406
1.3.1    2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000  price:$NaN)
1.3.2    2017-11-02 Taxable Gains (short-term) from sale of DASH 4.000000000: USD 788.113754
2.1      2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000  price:$NaN)
2.2      2017-11-02 Taxable Gains (short-term) from sale of BCH 0.358531680: USD -19.835594
3.1      2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000  price:$NaN)
3.2      2017-11-02 Taxable Gains (short-term) from sale of BTG 0.419883380: USD -10.481671
4        2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000     price:$NaN)
4.1      2017-11-02 Coinbase BTC 0.184032210  (basis:$1284.260719  price:$6978.456211)
4.1.1    2017-11-01 Coinbase USD 0.000000000  (basis:$0.000000     price:$NaN)
4.1.2    2017-11-01 Taxable Gains (short-term) from sale of BTC 0.000500000: USD -0.096380

=== Income: ===
2	2017-08-01 Bitfinex BCH 0.358531680	(basis:212.250000000,	price:$591.997895)
3	2017-10-23 Bitfinex BTG 0.419883380	(basis:57.386000000,	price:$136.671282)
(total income: $269.64)

=== Capital Gains: ===
1.1.1.2	2017-11-01 Taxable Gains (short-term) from sale of BTC 0.001000000: USD 5.409741
1.2.1.2	2017-11-01 Taxable Gains (short-term) from sale of ETH 0.010000000: USD 2.636406
1.3.2	2017-11-02 Taxable Gains (short-term) from sale of DASH 4.000000000: USD 788.113754
2.2	2017-11-02 Taxable Gains (short-term) from sale of BCH 0.358531680: USD -19.835594
3.2	2017-11-02 Taxable Gains (short-term) from sale of BTG 0.419883380: USD -10.481671
4.1.2	2017-11-01 Taxable Gains (short-term) from sale of BTC 0.000500000: USD -0.096380
(2017's capital gains: short-term:$765.75 long-term:$0.00)
(Total capital gains: short-term:$765.75 long-term:$0.00)

=== Account balances (and their lots): ===
Coinbase
	BTC 0.602905590 (basis:1859.691033	price:$3084.547670)
		1.1.1  2017-04-06 Coinbase BTC 0.418873380  (basis:$575.430314   price:$1373.757182)
		4.1    2017-11-02 Coinbase BTC 0.184032210  (basis:$1284.260719  price:$6978.456211)
	ETH 9.190000000 (basis:260.691223	price:$28.366836)
		1.2.1  2017-04-06 Coinbase ETH 9.190000000  (basis:$260.691223  price:$28.366836)
(Total basis: $2120.38)
(Total initial investment: $1085.00)

=== Present Value, Tab-Separated (to copy into spreadsheet): ===
lotName	account	currency	amount	costBasis	origPurchaseDate	daysSincePurchase	shortOrLongTerm	presentValue	unrealizedGainLoss	unrealizedGainLossPercent
1.1.1	Coinbase	BTC	0.418873380	575.43	2017-04-06	626	longTerm	1687.59	1112.16	193.3
1.2.1	Coinbase	ETH	9.190000000	260.69	2017-04-06	626	longTerm	1195.07	934.38	358.4
4.1	Coinbase	BTC	0.184032210	1284.26	2017-11-02	416	longTerm	741.45	-542.82	-42.3

`))
}

// simpleScenario creates a ledger and adds some simple activity to it.
// Then prints various reports (lots, capital gains, account balances)
func simpleScenario(w io.Writer) {
	l := ledger.New(USD, historicalPrices)

	// put up some money to invest, it has a higher cost-basis capturing transfer and deposit fees.
	l.DepositNewMoney(d("2017-04-06"), Bitfinex, 960, 1085 /* cost basis: $85 wire transfer + $40 deposit fee */)

	// use the investment money to purchase BTC
	l.Purchase(d("2017-04-06"), "1", Bitfinex, BTC, 0.83976678, 959+1.00 /*trade fee*/)

	// transfer the BTC to another account, cost basis is preserved
	l.Transfer(d("2017-11-01"), "1.1", BTC, 0.80000000, 0.001, Coinbase)

	//
	// Print results
	fmt.Fprintln(w, "=== Cost Basis Lots: ===")
	fmt.Fprintln(w, l.PrintLots())

	fmt.Fprintln(w, "=== Capital Gains: ===")
	fmt.Fprintln(w, l.PrintTaxableGains())

	fmt.Fprintln(w, "=== Account balances (and their lots): ===")
	fmt.Fprintln(w, l.PrintAccounts())

	fmt.Fprintln(w, "=== Present Value, Tab-Separated (to copy into spreadsheet): ===")
	fmt.Fprintln(w, l.PrintPresentValueTSV(d("2018-12-23"), map[ledger.Currency]float64{
		BTC: 4028.89,
	}))
}

// largerScenario creates a ledger and adds a variety of activity to it.
// Then prints various reports (lots, income, capital gains, account balances)
func largerScenario(w io.Writer) {
	l := ledger.New(USD, historicalPrices)

	l.DepositNewMoney(d("2017-04-06"), Bitfinex, 960, 1085 /* cost basis: $85 wire transfer + $40 deposit fee */)
	l.Purchase(d("2017-04-06"), "1", Bitfinex, BTC, 0.41988338, 503.35+1.00 /*trade fee*/)
	l.Purchase(d("2017-04-06"), "1", Bitfinex, ETH, 9.2000, 227.325+1.00 /*trade fee*/)
	l.Purchase(d("2017-04-06"), "1", Bitfinex, DASH, 4.000, 226.325+1.00 /*trade fee*/)

	l.Income(d("2017-08-01"), Bitfinex, BCH, 0.35853168, 212.25, "fork from BTC")
	l.Income(d("2017-10-23"), Bitfinex, BTG, 0.41988338, 57.386, "fork from BTC")

	// 11/1: transferred BTC, ETH to Coinbase
	l.Transfer(d("2017-11-01"), "1.1", BTC, 0.419883380, 0.001, Coinbase)
	l.Transfer(d("2017-11-01"), "1.2", ETH, 9.2000, 0.01, Coinbase)

	//// 11/2 Exchanged DASH, BCH, & BTG for BTC, which we treat as a taxable event
	l.ExchangeTaxable(d("2017-11-02"), "1.3", DASH, 4.000, 0, false, BTC, 0.15014768)   // after 0.0002018 BTC fee
	l.ExchangeTaxable(d("2017-11-02"), "2", BCH, 0.35853168, 0, false, BTC, 0.02764547) // after 0.00005701 BTC fee
	l.ExchangeTaxable(d("2017-11-02"), "3", BTG, 0.41988338, 0, false, BTC, 0.00673906) // after 0.00002753 BTC fee
	l.MergeIdenticalLots(d("2017-11-02"), BTC, []string{"1.3.1", "2.1", "3.1"})

	// 11/2 Transfer BTC from Bitfinex to Coinbase
	l.Transfer(d("2017-11-01"), "4", BTC, 0.18453221, 0.0005, Coinbase)

	// 12/1 Invent some random fee
	l.Fee(d("2017-12-01"), "1.1.1", BTC, 0.00001, "1.1.1")

	//
	// Print results
	fmt.Fprintln(w, "=== Lots: ===")
	fmt.Fprintln(w, l.PrintLots())

	fmt.Fprintln(w, "=== Income: ===")
	fmt.Fprintln(w, l.PrintIncome())

	fmt.Fprintln(w, "=== Capital Gains: ===")
	fmt.Fprintln(w, l.PrintTaxableGains())

	fmt.Fprintln(w, "=== Account balances (and their lots): ===")
	fmt.Fprintln(w, l.PrintAccounts())

	fmt.Fprintln(w, "=== Present Value, Tab-Separated (to copy into spreadsheet): ===")
	fmt.Fprintln(w, l.PrintPresentValueTSV(d("2018-12-23"), map[ledger.Currency]float64{
		BTC: 4028.89,
		ETH: 130.04,
	}))
}

func d(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return t
}
