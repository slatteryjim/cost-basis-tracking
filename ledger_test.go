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
			d("2017-11-02"): 6960.07,
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
1      2017-04-06 Bitfinex USD 0.000000000  (basis:$0.000000     price:$NaN)
1.1    2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689    price:$1292.025388)
1.1.1  2017-04-06 Coinbase BTC 0.799000000  (basis:$1033.620311  price:$1293.642441)

=== Capital Gains: ===
(Total capital gains: short-term:$0.00 long-term:$0.00)

=== Account balances (and their lots): ===
Bitfinex
	BTC 0.039766780 (basis:51.379689	price:$1292.025388)
		1.1  2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689  price:$1292.025388)
Coinbase
	BTC 0.799000000 (basis:1033.620311	price:$1293.642441)
		1.1.1  2017-04-06 Coinbase BTC 0.799000000  (basis:$1033.620311  price:$1293.642441)
(Total basis: $1085.00)
(Total initial investment: $1085.00)

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
1      2017-04-06 Bitfinex USD 0.000000000   (basis:$0.000000    price:$NaN)
1.1    2017-04-06 Bitfinex BTC 0.000000000   (basis:$0.000000    price:$NaN)
1.2    2017-04-06 Bitfinex ETH 0.000000000   (basis:$0.000000    price:$NaN)
1.3    2017-04-06 Bitfinex DASH 0.000000000  (basis:$0.000000    price:$NaN)
2      2017-08-01 Bitfinex BCH 0.000000000   (basis:$0.000000    price:$NaN)
3      2017-10-23 Bitfinex BTG 0.000000000   (basis:$0.000000    price:$NaN)
1.1.1  2017-04-06 Coinbase BTC 0.418873380   (basis:$570.020573  price:$1360.842202)
1.2.1  2017-04-06 Coinbase ETH 9.190000000   (basis:$258.054818  price:$28.079958)
1.3.1  2017-11-02 Bitfinex BTC 0.000000000   (basis:$0.000000    price:$NaN)
1.3.2  2017-11-02 Taxable Gains (short-term) from sale of DASH 4.000000000: USD 788.113754
2.1    2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000  price:$NaN)
2.2    2017-11-02 Taxable Gains (short-term) from sale of BCH 0.358531680: USD -19.835594
3.1    2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000  price:$NaN)
3.2    2017-11-02 Taxable Gains (short-term) from sale of BTG 0.419883380: USD -10.481671
4      2017-11-02 Bitfinex BTC 0.000000000  (basis:$0.000000     price:$NaN)
4.1    2017-11-02 Coinbase BTC 0.184032210  (basis:$1284.357099  price:$6978.979923)

=== Income: ===
2	2017-08-01 Bitfinex BCH 0.358531680	(basis:212.250000000,	price:$591.997895)
3	2017-10-23 Bitfinex BTG 0.419883380	(basis:57.386000000,	price:$136.671282)
(total income: $269.64)

=== Capital Gains: ===
1.3.2	2017-11-02 Taxable Gains (short-term) from sale of DASH 4.000000000: USD 788.113754
2.2	2017-11-02 Taxable Gains (short-term) from sale of BCH 0.358531680: USD -19.835594
3.2	2017-11-02 Taxable Gains (short-term) from sale of BTG 0.419883380: USD -10.481671
(2017's capital gains: short-term:$757.80 long-term:$0.00)
(Total capital gains: short-term:$757.80 long-term:$0.00)

=== Account balances (and their lots): ===
Coinbase
	BTC 0.602905590 (basis:1854.377672	price:$3075.734746)
		1.1.1  2017-04-06 Coinbase BTC 0.418873380  (basis:$570.020573   price:$1360.842202)
		4.1    2017-11-02 Coinbase BTC 0.184032210  (basis:$1284.357099  price:$6978.979923)
	ETH 9.190000000 (basis:258.054818	price:$28.079958)
		1.2.1  2017-04-06 Coinbase ETH 9.190000000  (basis:$258.054818  price:$28.079958)
(Total basis: $2112.43)
(Total initial investment: $1085.00)

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
}

func d(date string) time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return t
}
