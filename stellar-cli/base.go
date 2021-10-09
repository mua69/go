package main

import (
	"github.com/mua69/stellarwallet"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/clients/horizonclient"
	hprotocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"math/big"
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
	case hprotocol.Asset:
		if horizonclient.AssetType(a.Type) == horizonclient.AssetTypeNative {
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

func (a* Asset)HorizonType() horizonclient.AssetType {
	if a.isNative() {
		return horizonclient.AssetTypeNative
	} else {
		if len(a.code) <= 4 {
			return horizonclient.AssetType4
		} else {
			return horizonclient.AssetType12
		}
	}
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

func (a* Asset)toCreditAsset() txnbuild.Asset  {
	if a.isNative() {
		return txnbuild.NativeAsset{}
	} else {
		return txnbuild.CreditAsset{Code:a.Code(), Issuer:a.Issuer()}
	}
}


func (a *Asset)toBuildAsset() txnbuild.Asset {
	if a.isNative() {
		return txnbuild.NativeAsset{}
	} else {
		return txnbuild.CreditAsset{a.code, a.issuer}
	}
}

func assetsToOrderBookReuqest( selling, buying *Asset) horizonclient.OrderBookRequest {
	var r horizonclient.OrderBookRequest

	r.SellingAssetCode = selling.Code()
	r.SellingAssetIssuer = selling.Issuer()
	r.SellingAssetType = selling.HorizonType()

	r.BuyingAssetCode = buying.Code()
	r.BuyingAssetIssuer = buying.Issuer()
	r.BuyingAssetType = buying.HorizonType()

	return r
}


func amountToRat(a string) *big.Rat {
	amnt := amount.MustParse(a)
	return big.NewRat(int64(amnt), 1)
}

func amountToString(a *big.Rat) string {
	r := big.Rat{}
	r.Quo(a, big.NewRat(amount.One, 1))

	return r.FloatString(7)
}

func amountToStringPretty(a *big.Rat) string {
	r := big.Rat{}
	r.Quo(a, big.NewRat(amount.One, 1))

	return r.FloatString(2)
}


