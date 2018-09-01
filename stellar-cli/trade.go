package main

import (
	"github.com/mua69/stellarwallet"
	"github.com/stellar/go/clients/horizon"
	"math/big"
	"github.com/stellar/go/amount"
	"fmt"
	"github.com/stellar/go/build"
	"github.com/stellar/go/keypair"
	"time"
)

type Asset struct {
	code string
	issuer string
}

var g_assets map[string]*Asset
var g_nativeAsset *Asset

func newAsset(issuer, code string) *Asset {
	key := issuer + "/" + code

	if g_assets == nil {
		g_assets = make(map[string]*Asset)
	}

	a := g_assets[key]

	if a == nil {
		a = &Asset{code, issuer}
		g_assets[key] = a
	}

	return a
}

func newNativeAsset() *Asset {
	if g_nativeAsset == nil {
		g_nativeAsset = &Asset{}
	}

	return g_nativeAsset
}

func newAssetFrom(a interface{}) *Asset {
	switch a := a.(type) {
	case *stellarwallet.Asset:
		if a == nil {
			return newNativeAsset()
		} else {
			return newAsset(a.Issuer(), a.AssetId())
		}
	case horizon.Asset:
		if a.Type == "native" {
			return newNativeAsset()
		} else {
			return newAsset(a.Issuer, a.Code)
		}
	}

	return nil
}

func (a *Asset)isNative() bool {
	if a.issuer == "" {
		return true
	}
	return false
}

func (a1 *Asset)isEqual(a2 *Asset) bool {
	if a1.isNative() && a2.isNative() {
		return true
	}

	if a1.issuer == a2.issuer && a1.code == a2.code {
		return true
	}

	return false
}

func (a* Asset)Issuer() string {
	return a.issuer
}

func (a* Asset)Code() string {
	return a.code
}

func (a* Asset)String() string {
	if a.isNative() {
		return "XLM"
	}

	return a.code +  "/" + a.issuer
}

func (a* Asset)StringPretty() string {
	if a.isNative() {
		return "XLM"
	}

	issuer := a.issuer[:5] + "..." + a.issuer[len(a.issuer)-5:]

	return a.code +  "/" + issuer
}

func (a* Asset)codeToString() string {
	if a.isNative() {
		return "XLM"
	} else {
		return a.code
	}
}

func (a* Asset)toHorizonAsset() horizon.Asset {
	var ha horizon.Asset

	if a.isNative() {
		ha.Type = "native"
	} else {
		if len(a.code) <= 4 {
			ha.Type = "credit_alphanum4"
		} else {
			ha.Type = "credit_alphanum12"
		}
		ha.Code = a.code
		ha.Issuer = a.issuer
	}

	return ha
}

func (a *Asset)toBuildAsset() build.Asset {
	if a.isNative() {
		return build.NativeAsset()
	} else {
		return build.CreditAsset(a.code, a.issuer)
	}
}

func printOrderBook(asset1, asset2 *Asset, maxLines int, timeout time.Duration) {
	ob, err := getOrderBook(asset1, asset2, timeout)

	if err != nil {
		printHorizonError("Load Order Book", err)
		return
	}

	var table [][]string

	codeBuying := newAssetFrom(ob.Buying).codeToString()
	codeSelling := newAssetFrom(ob.Selling).codeToString()

	table = appendTableLine(table, codeBuying, codeSelling, "Bid", "Ask", codeSelling, codeBuying)

	nb := len(ob.Bids)
	na := len(ob.Asks)
	n := nb

	if na > nb {
		n = na
	}

	if n > maxLines {
		n = maxLines
	}

	for i:=0; i < n; i++ {
		ba1 := ""
		ba2 := ""
		bp := ""
		aa1 := ""
		aa2 := ""
		ap := ""

		if i < nb {
			p := big.NewRat(int64(ob.Bids[i].PriceR.N), int64(ob.Bids[i].PriceR.D))
			a2 := big.NewRat(int64(amount.MustParse(ob.Bids[i].Amount)), 1)
			a2.Quo(a2, p)
			ba2 = amountToString(a2)

			ba1 = ob.Bids[i].Amount
			bp = ob.Bids[i].Price
		}

		if i < na {
			p := big.NewRat(int64(ob.Asks[i].PriceR.N), int64(ob.Asks[i].PriceR.D))
			a1 := big.NewRat(int64(amount.MustParse(ob.Asks[i].Amount)), 1)
			a1.Mul(a1, p)
			aa1 = amountToString(a1)

			aa2 = ob.Asks[i].Amount
			ap = ob.Asks[i].Price
		}

		table = appendTableLine(table, ba1, ba2, bp, ap, aa2, aa1)
	}

	printTable(table, 6, " ")
}

// calculate average price to sell/buy 'amount' of asset1 for asset2 based on current order book
// price in asset/asset1
func getAverageAssetPrice(asset1, asset2 *Asset, amnt *big.Rat, timeout time.Duration) (buyPrice, sellPrice *big.Rat) {
	ob, err := getOrderBook(asset1, asset2, timeout)

	if err != nil {
		printHorizonError("Load Order Book", err)
		return
	}

	//fmt.Printf("%s->%s\n", asset1.StringPretty(), asset2.StringPretty())

	zero := big.NewRat(0, 1)
	buyAmount := big.NewRat(0, 1)
	sellAmount := big.NewRat(0, 1)
	amountCount := &big.Rat{}
	amountCount.Set(amnt)

	for i := range(ob.Bids) {
		p := big.NewRat(int64(ob.Bids[i].PriceR.N), int64(ob.Bids[i].PriceR.D))
		a2 := big.NewRat(int64(amount.MustParse(ob.Bids[i].Amount)), 1) // amount of asset2
		a1 := &big.Rat{}
		a1.Quo(a2, p) // amount of asset1

		//fmt.Printf("Bid: a1: %s, a2: %s: p: %s\n", amountToString(a1), amountToString(a2), p.FloatString(3))
		if amountCount.Cmp(a1) >= 0 {
			//fmt.Printf("Bid 1\n")
			sellAmount.Add(sellAmount, a2)
			amountCount.Sub(amountCount, a1)
		} else {
			a2.Mul(amountCount, p)
			sellAmount.Add(sellAmount, a2)
			//fmt.Printf("Bid 2: a1: %s, a2: %s\n", amountToString(amountCount), amountToString(a2))
			amountCount.Set(zero)
		}
	}
	amountCount.Sub(amnt, amountCount) // sold amount
	sellPrice = &big.Rat{}
	if amountCount.Cmp(zero) > 0 {
		sellPrice.Quo(sellAmount, amountCount)
	} else {
		sellPrice.Set(zero)
	}

	//fmt.Printf("Avg sell price for %s %s: %s %s\n", amountToString(amountCount), asset1.StringPretty(),
	//	sellPrice.FloatString(6), asset2.StringPretty())

	amountCount.Set(amnt)
	for i := range(ob.Asks) {
		p := big.NewRat(int64(ob.Asks[i].PriceR.N), int64(ob.Asks[i].PriceR.D))
		a1 := big.NewRat(int64(amount.MustParse(ob.Asks[i].Amount)), 1) // amount of asset1
		a2 := &big.Rat{}
		a2.Mul(a1, p) // amount of asset2

		//fmt.Printf("Ask: a1: %s, a2: %s: p: %s\n", amountToString(a1), amountToString(a2), p.FloatString(3))

		if amountCount.Cmp(a1) >= 0 {
			//fmt.Printf("Ask 1\n")
			buyAmount.Add(buyAmount, a2)
			amountCount.Sub(amountCount, a1)
		} else {
			a2.Mul(amountCount, p)
			buyAmount.Add(buyAmount, a2)
			//fmt.Printf("Ask 2: a1: %s, a2: %s\n", amountToString(amountCount), amountToString(a2))
			amountCount.Set(zero)
		}
	}

	amountCount.Sub(amnt, amountCount)
	buyPrice = &big.Rat{}
	if amountCount.Cmp(zero) > 0 {
		buyPrice.Quo(buyAmount, amountCount)
	} else {
		buyPrice.Set(zero)
	}

	//fmt.Printf("Avg buy price for %s %s: %s %s\n", amountToString(amountCount), asset1.StringPretty(),
	//	buyPrice.FloatString(6), asset2.StringPretty())

	return
}


type Offer struct {
	orderid uint64
	buying bool
	asset1 *Asset
	asset2 *Asset
	price *big.Rat
	amount1 *big.Rat
	amount2 *big.Rat
}

func (offer *Offer)reverse() {
	offer.buying = !offer.buying
	offer.asset1, offer.asset2 = offer.asset2, offer.asset1
	offer.amount1, offer.amount2 = offer.amount2, offer.amount1
	offer.price.Inv(offer.price)
}

func (offer *Offer)string() string {
	var s1, s2 string

	if offer.buying {
		s1 = "Buy "
		s2 = "with"
	} else {
		s1 = "Sell"
		s2 = "for"
	}

	return fmt.Sprintf("%s: %s %s %s %s %s, price %s, ID %d", s1,
			amountToString(offer.amount1), offer.asset1.StringPretty(),
			s2,
			amountToString(offer.amount2), offer.asset2.StringPretty(),
			offer.price.FloatString(7), offer.orderid)
}

func getOffers(account string, asset1, asset2 *Asset) []*Offer {
	matchAll := false

	if asset1 == nil || asset2 ==  nil {
		matchAll = true
	}

	offers, err := g_horizon.LoadAccountOffers(account)

	if err != nil {
		printHorizonError("Load Account Offers", err)
		return nil
	}

	res := make([]*Offer, 0, len(offers.Embedded.Records))

	for i, _ := range offers.Embedded.Records {
		o := &offers.Embedded.Records[i]

		offer := &Offer{}
		offer.orderid = uint64(o.ID)
		offer.buying = false
		offer.asset1 = newAssetFrom(o.Selling)
		offer.asset2 = newAssetFrom(o.Buying)
		offer.price = priceToRat(o.PriceR)
		offer.amount1 = amountToRat(o.Amount)
		offer.amount2 = amountToRat(o.Amount)
		offer.amount2.Mul(offer.amount2,offer.price)

		if matchAll {
			if g_wallet != nil {
				// check if we find a trading pair in the wallet that indicates an reversed asset order
				var a1, a2 *stellarwallet.Asset

				if !offer.asset1.isNative() {
					a1 = g_wallet.FindAsset(offer.asset1.issuer, offer.asset1.code)
				}
				if !offer.asset2.isNative() {
					a2 = g_wallet.FindAsset(offer.asset2.issuer, offer.asset2.code)
				}
				if (offer.asset1.isNative() || a1 != nil) && (offer.asset2.isNative() || a2 != nil) {
					if g_wallet.FindTradingPair(a2, a1) != nil {
						offer.reverse()
					}
				}
			}

			res = append(res, offer)
		} else if asset1.isEqual(offer.asset1) && asset2.isEqual(offer.asset2) {
			res = append(res, offer)
		} else if asset1.isEqual(offer.asset2) && asset2.isEqual(offer.asset1) {
			offer.reverse()
			res = append(res, offer)
		}
	}

	return res
}

func printOffers(offers []*Offer) {
	for _,o := range offers {
		fmt.Println(o.string())
	}
}

func selectOffer(prompt string, offers []*Offer) uint64 {
	menu := make([]MenuEntry, 0, len(offers)+1)

	type choiceType struct {
		id string
		orderid uint64
	}

	choices := make([]choiceType, 0, len(offers)+1)

	choice := 1

	for _, o:= range offers {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, o.string(), true})
		choices = append(choices, choiceType{s, o.orderid})
		choice++
	}

	s := fmt.Sprintf("%d", choice)
	menu = append(menu, MenuEntry{ s, "Done", true})
	choices = append(choices, choiceType{s, 0})
	choice++

	fmt.Printf("\n%s:\n", prompt)
	sel := runMenu(menu, false)

	for i,_ := range choices {
		if choices[i].id == sel {
			return choices[i].orderid
		}
	}

	return 0
}

func trade() {
	acc, src, tx := enterSourceAccount()

	srcPub := keypair.MustParse(src).Address()

	asset1, asset2 := enterTradingPair("Trading Pair")

	intendedAmount := getAmount("Intended trade amount")

	code1 := asset1.codeToString()
	code2 := asset2.codeToString()

	var orderid uint64
	orderid = 0

	fmt.Printf("\n%s <=> %s\n", asset1.StringPretty(), asset2.StringPretty())

	for {

		menu := []MenuEntry{
			{ "buy", fmt.Sprintf("Buy %s with %s", code1, code2), true},
			{ "sell", fmt.Sprintf("Sell %s for %s", code1, code2), true},
			{ "orderid", fmt.Sprintf("Enter Order ID (current: %d)", orderid), true},
			{ "update", "Update Order Book", true},
			{ "done", "Done", true}}

		fmt.Println()
		fmt.Println("Order Book:")
		printOrderBook(asset1, asset2, 3, CacheTimeoutForce)

		if intendedAmount.Cmp(gRatZero) > 0 {
			buyPrice, sellPrice := getAverageAssetPrice(asset1, asset2, intendedAmount, CacheTimeoutShort)
			if buyPrice != nil && sellPrice != nil {
				fmt.Printf("\nAverage buy/sell price for %s %s: %s/%s\n", amountToStringPretty(intendedAmount),
					code1, buyPrice.FloatString(7), sellPrice.FloatString(7))
			}
		}

		fmt.Println("\nOffers:")
		offers := getOffers(srcPub, asset1, asset2)
		if len(offers) > 0 {
			printOffers(offers)
		} else {
			fmt.Println("no offers")
		}

		fmt.Println("\nSelect action:")
		sel := runMenu(menu, false)

		if sel == "done" {
			return
		}

		if sel == "update" {
			continue
		}

		if sel == "orderid" {
			orderid = selectOffer("Select Offer:", getOffers(srcPub, asset1, asset2))
			continue
		}

		rate := getPrice("Price")
		amount1 := getAmount("Amount")

		amount2 := &big.Rat{}
		amount2.Mul(amount1, rate)

		rateInv := &big.Rat{}
		rateInv.Inv(rate)

		amount3 := &big.Rat{}
		amount3.Mul(amount2, rateInv)

		if sel == "sell" {
			fmt.Printf("Selling %s %s for %s %s, rate %s\n", amountToString(amount1), code1,
				amountToString(amount2), code2, rate.FloatString(7))

			tx_addOrder(tx, asset1, asset2, rate, amount1, orderid)
		} else {
			fmt.Printf("Buying %s %s with %s %s, rate %s\n", amountToString(amount1), code1,
				amountToString(amount2), code2, rate.FloatString(7))
			fmt.Printf("Selling %s %s for %s %s, rate %s\n", amountToString(amount2), code2,
				amountToString(amount3), code1, rateInv.FloatString(7))

			tx_addOrder(tx, asset2, asset1, rateInv, amount2, orderid)
		}

		transactionFinalize(acc, src, tx)

		tx = tx_setup(src)
	}
}
