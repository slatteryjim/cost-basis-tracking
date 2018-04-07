# Cost Basis Tracking
Go code to track the cost-basis of your cryptocurrency transactions.
Help reduce headaches when filing your taxes.

I created this library after trying several existing accounting tools and finding them to be lacking:
- [bitcoin.tax](https://bitcoin.tax/)
  This was pretty good, but did not factor in fees as I would have hoped.
- [Ledger-cli](https://www.ledger-cli.org/) and other CLI accounting tools
- [GNUCash](https://gnucash.org/) did not maintain cost basis lots when transferring money between funds.
It also didn't capture fees as I would have hoped.

# Example use case
You wired some money to a cryptocurrency exchange, 
bought some cryptocurrencies, 
did some exchanges, 
and transferred the coins out.

You want to:
- Create cost basis lots for your purchases
- Be sure your original wire transfer fees and deposit fees are seamlessly factored into your cost basis
- Record short-term/long-term gains when exchanging one cyrptocurrency for another
- Factor in transfer fees when transferring the coins out to another account

# Usage
For now, you write Go code to express your transactions and can see reports printed out:
- List all of you cost basis lots
- Summarize all of your accounts and their holdings, broken down by cost basis lots
- See all of your taxable income (from cryptocurrency forks or airdrops)
- See all of your capital gains (short-term and long-term)

## Cost basis lots
Store the date acquired, currency type and amount, and the cost basis (how much money was spent acquiring this lot,
including any fees).

Cost basis lots are automatically named with an automatically incrementing number (e.g. `"1"`, `"2"`, ...).

As lots are transferred or exchanged, child lots are created and given a name based on the parent lot 
(e.g. `"1.1"`, `"1.2"`) so you can get a sense of the hierarchy just from the name.

# Sample
See [ledger_test.go](ledger_test.go)

```go
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
```

Output:
```
=== Cost Basis Lots: ===
1      2017-04-06 Bitfinex USD 0.000000000  (basis:$0.000000     price:$NaN)
1.1    2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689    price:$1292.025388)
1.1.1  2017-04-06 Coinbase BTC 0.799000000  (basis:$1033.620311  price:$1293.642441)

=== Capital Gains: ===
(total short-term gains: $0.00)
(total long-term gains:  $0.00)

=== Account balances (and their lots): ===
Bitfinex
	BTC 0.039766780 (basis:51.379689	price:$1292.025388)
		1.1  2017-04-06 Bitfinex BTC 0.039766780  (basis:$51.379689  price:$1292.025388)
Coinbase
	BTC 0.799000000 (basis:1033.620311	price:$1293.642441)
		1.1.1  2017-04-06 Coinbase BTC 0.799000000  (basis:$1033.620311  price:$1293.642441)
(Total basis: $1085.00)
(Total initial investment: $1085.00)
```

See [ledger_test.go TestLargerScenario](ledger_test.go) for a more complex scenario with income and capital gains.
