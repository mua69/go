package main

import (
	"encoding/json"
	"fmt"
	"github.com/mua69/stellarwallet"
	"github.com/pkg/errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/price"
	hprotocol "github.com/stellar/go/protocols/horizon"
	"math/big"
	"time"
)

const CacheTimeoutForce = time.Duration(0) // force refresh
const CacheTimeoutShort = time.Duration(30) // 30 sec
const CacheTimeoutMedium = time.Duration(120) // 2 minutes
const CachTimeoutLong = time.Duration(600) // 10 minutes

type AccountSigner struct {
	id string
	weight uint32
	stype string
}

type AccountInfo struct {
	id string
	exists bool
	timestamp time.Time
	sequence string
	horizonData *hprotocol.Account
	account *stellarwallet.Account
	balances map[*Asset]*big.Rat
	signers []AccountSigner
}

var g_accountInfoCache = make(map[string]*AccountInfo)

func getAccountInfo(id string, timeout time.Duration) *AccountInfo {
	d := g_accountInfoCache[id]

	if d != nil {
		if time.Since(d.timestamp) < timeout * time.Second {
			return d
		}
	}

	//fmt.Printf("Reloading account info for %s\n", id)

	delete(g_accountInfoCache, id)

	d = new(AccountInfo)
	d.id = id

	acc, err := g_horizon.AccountDetail(horizonclient.AccountRequest{ AccountID:id })

	if err != nil {
		if herr, ok := err.(*horizonclient.Error); ok {
			if herr.Problem.Title == "Resource Missing" {
				d.exists = false
			} else {
				d = nil
			}
		} else {
			d = nil
		}

		if d == nil {
			printHorizonError("load account", err)
			return nil
		}
	} else {
		d.exists = true
	}

	if g_wallet != nil {
		d.account = g_wallet.FindAccountByPublicKey(id)
	}

	if d.exists {
		d.horizonData = &acc
		d.sequence = acc.Sequence
		d.balances = make(map[*Asset]*big.Rat)

		for _, b := range acc.Balances {
			var a *Asset
			if b.Asset.Code != "" {
				a = newAsset(b.Asset.Issuer, b.Asset.Code)
			} else {
				a = newNativeAsset()
			}
			d.balances[a] = amountToRat(b.Balance)
		}

		for _, signer := range acc.Signers {
			d.signers = append(d.signers, AccountSigner{signer.Key, uint32(signer.Weight), signer.Type})
		}
	}

	d.timestamp = time.Now()

	g_accountInfoCache[id] = d
	return d
}

func (a *AccountInfo) getNativeBalance() *big.Rat {
	v := a.balances[newNativeAsset()]

	if v != nil {
		return v
	}

	return big.NewRat(0, 1)
}

func clearAccountInfoCache(id string) {
	if id != "" {
		delete(g_accountInfoCache, id)
	} else {
		g_accountInfoCache = make(map[string]*AccountInfo)
	}
}

type AssetPair struct {
	a1, a2 *Asset
}

type OrderBookCache struct {
	ob *hprotocol.OrderBookSummary
	timestamp time.Time
}

var gOrderBookCache = make(map[AssetPair]*OrderBookCache)

func getOrderBook(asset1, asset2 *Asset, timeout time.Duration) (*hprotocol.OrderBookSummary, error) {
	p := AssetPair{asset1, asset2}

	ent := gOrderBookCache[p]

	if ent != nil {
		if time.Since(ent.timestamp) < timeout*time.Second {
			return ent.ob, nil
		}
	}

	fmt.Printf("loading order book for asset pair: %s %s ...\n", asset1.StringPretty(), asset2.StringPretty())

	ob, err := g_horizon.OrderBook(assetsToOrderBookReuqest(asset1, asset2))

	if err != nil {
		return nil, err
	}

	if ent == nil {
		ent = new(OrderBookCache)
	}

	ent.ob = &ob
	ent.timestamp = time.Now()

	gOrderBookCache[p] = ent

	return ent.ob, nil
}

func getAccountTransactions(adr string, cnt int, pagingTokenIn string) (txs []hprotocol.Transaction, pagingTokenOut string, err error) {
	if cnt <= 0 {
		return
	}

	acc := getAccountInfo(adr, CacheTimeoutMedium)

	if !acc.exists {
		return
	}

	if cnt > 200 {
		cnt = 200 // maximum limit allowed by horizon server
	}

	txp, err := g_horizon.Transactions(horizonclient.TransactionRequest{ForAccount:adr,
		Limit:uint(cnt), Cursor:pagingTokenIn, Order:horizonclient.OrderDesc})

	if err != nil {
		printHorizonError("get account transactions", err)
		return
	}

	txs = txp.Embedded.Records

	n := len(txs)

	if n == 0 {
		return
	}

	if n >= cnt {
		// there are probably more transaction records, return paging token of last transaction
		pagingTokenOut = txs[n-1].PT
	}

	return
}

func getAccountTrades(adr string, cnt int, pagingTokenIn string) (trades []hprotocol.Trade, pagingTokenOut string, err error) {
	if cnt <= 0 {
		return
	}

	acc := getAccountInfo(adr, CacheTimeoutMedium)

	if !acc.exists {
		return
	}

	if cnt > 200 {
		cnt = 200 // maximum limit allowed by horizon server
	}

	tp, err := g_horizon.Trades(horizonclient.TradeRequest{ForAccount:adr,
		Limit:uint(cnt), Cursor:pagingTokenIn, Order:horizonclient.OrderDesc})

	if err != nil {
		printHorizonError("get account transactions", err)
		return
	}

	trades = tp.Embedded.Records

	n := len(trades)

	if n == 0 {
		return
	}

	if n >= cnt {
		// there are probably more transaction records, return paging token of last transaction
		pagingTokenOut = trades[n-1].PT
	}

	return
}

type ReferenceCurrencyPrice struct {
	name string // name of reference currency
	price *big.Rat // price in 'ref currency'/XLM
}

type ReferenceCurrencyCache struct {
	price *ReferenceCurrencyPrice
	timestamp time.Time
}

var gReferenceCurrencyCache ReferenceCurrencyCache


func getReferenceCurrencyPrice(timeout time.Duration) *ReferenceCurrencyPrice {
	if gReferenceCurrency == ReferenceCurrencyNone {
		return nil
	}

	if gReferenceCurrencyCache.price != nil {
		if time.Since(gReferenceCurrencyCache.timestamp) < timeout * time.Second {
			return gReferenceCurrencyCache.price
		}
	}

	fmt.Printf("Fetching reference currency price...\n")

	gReferenceCurrencyCache.price = getRefCurrPriceKraken()

	if gReferenceCurrencyCache.price == nil {
		gReferenceCurrencyCache.price = getRefCurrPriceSDEX()
	}

	if gReferenceCurrencyCache.price != nil {
		gReferenceCurrencyCache.timestamp = time.Now()
		return gReferenceCurrencyCache.price
	}

	return nil
}

var gSdexReferenceCurrencyAsset = newAsset("GAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S", "EURT")

func getRefCurrPriceSDEX() *ReferenceCurrencyPrice {
	_, sellPrice := getAverageAssetPrice(newNativeAsset(), gSdexReferenceCurrencyAsset, big.NewRat(1000, 1),
		CacheTimeoutForce)
	zero := big.NewRat(0, 1)
	if sellPrice.Cmp(zero) > 0 {
		return &ReferenceCurrencyPrice{gSdexReferenceCurrencyAsset.Code(), sellPrice}
	} else {
		return nil
	}
}

func getRefCurrPriceKraken() *ReferenceCurrencyPrice {
	url := "https://api.kraken.com/0/public/Ticker?pair="

	var name string

	switch gReferenceCurrency {
	case ReferenceCurrencyNone:
		return nil
	case ReferenceCurrencyEUR:
		url += "XLMEUR"
		name = "EUR"
	case ReferenceCurrencyUSD:
		url += "XLMUSD"
		name = "USD"
	case ReferenceCurrencyBTC:
		url += "XLMXBT"
		name = "BTC"
	}

	type KrakenJsonTickerData struct {
		Ask []string `json:"a"`
		Bid []string `json:"b"`
		Avg []string `json:"p"`
	}
	type krakenJsonData struct {
		Error []string `json:"error"`
		Result map[string]KrakenJsonTickerData `json:"result"`
	}
	
	var data krakenJsonData

	err := urlToJson(url, &data)

	if err != nil {
		fmt.Printf("url2Json failed: %s\n", err.Error())
		return nil
	}
	
	if len(data.Error) > 0 {
		fmt.Printf("Kraken: error: %s\n", data.Error[0])
		return nil
	}

	for pair, ticker := range data.Result {
		fmt.Printf("Kraken: debug: pair: %s\n", pair)
		if len(ticker.Avg) > 0 {
			p, err := price.Parse(ticker.Avg[0])
			if err != nil {
				fmt.Printf("Kraken: parsing price failed: %s", err.Error())
			} else {
				return &ReferenceCurrencyPrice{name, big.NewRat(int64(p.N), int64(p.D))}
			}
		}
	}

	return nil
}

func urlToJson(url string, data interface{}) error {
	resp, err := g_horizon.HTTP.Get(url)
	if err != nil {
		return errors.Wrap(err, "get url: "+url)
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return errors.New(fmt.Sprintf("get url: %s: failed: %s", url, resp.Status))
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)

	err = decoder.Decode(data)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("url: %s: json decoding failed", url))
	}

	return nil
}