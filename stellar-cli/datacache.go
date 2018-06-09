package main

import (
	"github.com/stellar/go/clients/horizon"
	"math/big"
	"github.com/mua69/stellarwallet"
)

type AccountSigner struct {
	id string
	weight uint32
}

type AccountInfo struct {
	id string
	exists bool
	horizonData *horizon.Account
	account *stellarwallet.Account
	balances map[*Asset]*big.Rat
	signers []AccountSigner
}

var g_accountInfoCache = make(map[string]*AccountInfo)

func getAccountInfo(id string, force bool) *AccountInfo {
	d := g_accountInfoCache[id]

	if d != nil && !force {
		return d
	}

	delete(g_accountInfoCache, id)

	d = new(AccountInfo)
	d.id = id

	acc, err := g_horizon.LoadAccount(id)

	if err != nil {
		if herr, ok := err.(*horizon.Error); ok {
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
			d.signers = append(d.signers, AccountSigner{signer.Key, uint32(signer.Weight)})
		}
	}

	g_accountInfoCache[id] = d
	return d
}

func clearAccountInfoCache(id string) {
	if id != "" {
		delete(g_accountInfoCache, id)
	} else {
		g_accountInfoCache = make(map[string]*AccountInfo)
	}
}
