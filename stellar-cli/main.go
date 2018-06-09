package main

import (
	"fmt"
	"flag"
	"strings"
	"net/http"
	"io/ioutil"
	"os"
	"encoding/hex"
	"sync"
	"time"

	"github.com/mua69/stellarwallet"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/build"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/clients/stellartoml"
	"github.com/stellar/go/clients/federation"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/network"
	"math/big"
)


var (
	g_network= build.TestNetwork
	g_horizon = horizon.DefaultTestNetClient
	g_online = true
	g_version = "1.0.0"
	g_gitHash = "undefined"

	g_signers []string

	// wallet related global variables
	g_wallet *stellarwallet.Wallet
	g_walletPath string
	g_walletPassword string
	g_walletPasswordLock = 0
	g_walletPasswordLockMutex sync.Mutex
	g_walletPasswordLockDuration = 120 // password lock duration in seconds
	g_walletPasswordUnlockTime time.Time

	// command line flags
	g_txIn = ""
	g_txOut = ""
	g_signersFile = ""
	g_horizonUrl = ""
	g_testnet = false
	g_noWallet bool
)


func clearSigners() {
	for i, _ := range g_signers {
		stellarwallet.EraseString(&g_signers[i])
	}

	g_signers = nil
}

func setupNetwork() {
	if g_testnet {
		g_network = build.TestNetwork
		g_horizon = horizon.DefaultTestNetClient
	} else {
		g_network = build.PublicNetwork 	
		g_horizon = &horizon.Client{
			URL:  "https://horizon.stellarport.earth/",
			HTTP: http.DefaultClient,
		}
	}
			

	if g_horizonUrl != "" {
		g_horizon.URL = g_horizonUrl
	}

	fmt.Println("Using Network       :", g_network)
	fmt.Println("Using Horizon Server:", g_horizon.URL)
	fmt.Println()
}


func newKeyPair(pattern string, pos int) {

	var kp *keypair.Full

	if pattern == "" {
		kp, _ = keypair.Random()
	} else {	
		pattern = strings.ToUpper(pattern)

		if pos == 0 {
			// search address containing given pattern with no position restriction
			for {
				kp, _ = keypair.Random()
				index := strings.Index(kp.Address(), pattern)
				if index >= 0 { break }
			}
		} else if pos > 0 {
			// search address containing given pattern within pos first characters
			if pos == 1 && pattern[0] != 'G' {
				pos = 2
			}
			
			for {
				kp, _ = keypair.Random()
				index := strings.Index(kp.Address(), pattern)
				if index >= 0 && index < pos { break }
			}
		} else {
			// search address containing given pattern within pos last characters
			pos *= -1
			if pos < len(pattern) {
				pos = len(pattern)
			}

			pos = 56 - pos 

			for {
				kp, _ = keypair.Random()
				index := strings.LastIndex(kp.Address(), pattern)
				if index >= 0 && index >= pos { break }
			}
		}
	}

	fmt.Println("Address    :", kp.Address())
	fmt.Println("Private Key:", kp.Seed())
}

func getFund(adr string) {
    resp, err := http.Get("https://horizon-testnet.stellar.org/friendbot?addr=" + adr)
    if err != nil {
        panic(err)
    }

    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(body))
}

func federationLookup(adr string) ( id, memoType, memo string)  {
	client := &federation.Client{
		HTTP:        http.DefaultClient,
		Horizon:     g_horizon,
		StellarTOML: stellartoml.DefaultClient,
	}

	r, err := client.LookupByAddress(adr)

	if err == nil {
		id = r.AccountID
		memoType = r.MemoType
		memo = r.Memo.String()
	}

	return
}

func showBalances() {
	balances := make(map[*Asset]*big.Rat)
	xlmBalance := big.NewRat(0, 1)

	for _, a := range g_wallet.SeedAccounts() {
		info := getAccountInfo(a.PublicKey(), true)
		if info != nil {
			for asset, b := range info.balances {
				if asset.isNative() {
					xlmBalance.Add(xlmBalance, b)
				} else {
					if balances[asset] == nil {
						balances[asset] = big.NewRat(0, 1)
					}
					balances[asset].Add(balances[asset], b)
				}
			}
		}
	}

	var table [][]string

	maxLen := len(amountToString(xlmBalance))
	for _, b := range balances {
		l := len(amountToString(b))
		if l > maxLen {
			maxLen = l
		}
	} 


	table = appendTableLine(table, "XLM", fmt.Sprintf("%*s", maxLen, amountToString(xlmBalance)))
	
	for a, b := range balances {
		table = appendTableLine(table, a.StringPretty(), fmt.Sprintf("%*s", maxLen, amountToString(b)))
	} 

	fmt.Println("Balances:")
	printTable(table, 2, ": ")
}

func accountInfo(adr string) {
	var table [][]string

	
	acc, err := loadAccount(adr)
	
	if err != nil {
		panic(err)
	}

	if acc == nil {
		fmt.Println("Account does not exist:", adr)
		return
	}	

	table = appendTableLine(table, "Address", adr)

	maxBalanceStringLen := 0
	for i, _ := range acc.Balances {
		as := &acc.Balances[i]
		if len(as.Balance) > maxBalanceStringLen {
			maxBalanceStringLen = len(as.Balance)
		}
	}

	table = appendTableLine(table, "Balance (XLM)", fmt.Sprintf("%*s", maxBalanceStringLen,
		acc.GetNativeBalance()))
	for i, _ := range acc.Balances {
		as := &acc.Balances[i]
		if as.Asset.Code != "" {
			table = appendTableLine(table, fmt.Sprintf("Balance (%s)", as.Asset.Code),
				fmt.Sprintf("%*s (%s/%s)", maxBalanceStringLen, as.Balance, as.Asset.Code, as.Asset.Issuer))
		}
	}
	table = appendTableLine(table, "Inflation Destination", acc.InflationDestination)
	if acc.HomeDomain != "" {
		table = appendTableLine(table, "Home Domain", acc.HomeDomain)
	}
	
	var flags []string

	if acc.Flags.AuthRequired {
		flags = append(flags, "AUTH_REQUIRED")
	}
	if acc.Flags.AuthRevocable {
		flags = append(flags, "AUTH_REVOCABLE")
	}

	if len(flags) > 0 {
		table = appendTableLine(table, "Account Flags", strings.Join(flags, " "))
	}

	table = appendTableLine(table, "Thresholds Low/Med/High",
		fmt.Sprintf("%d/%d/%d", acc.Thresholds.LowThreshold, acc.Thresholds.MedThreshold,
			acc.Thresholds.HighThreshold))
	
	for _, signer := range acc.Signers {
		//table = appendTableLine(table, "Signer", fmt.Sprintf("%s Weight:%d Key:%s Type:%s", signer.PublicKey,
		//	signer.Weight, signer.Key, signer.Type))
		table = appendTableLine(table, "Signer", fmt.Sprintf("%s Weight:%d", signer.Key,
			signer.Weight))
	}
	
	printTable(table, 2, ": ")
}


func transactionFinalize(acc *stellarwallet.Account, src string, tx *build.TransactionBuilder) {
	tx_finalize(tx)

	signed, txe := enterSigners(acc, src, tx)

	fmt.Println("\nTransaction summary:")
	
	print_transaction(txe.E, "", os.Stdout)

	fmt.Println("\n")

	if signed {
		if getOk("Transmit transaction") {
			tx_transmit(txe)
		} else {
			fmt.Println("Transaction aborted.")
		}
	} else {
		fmt.Println("No signing key provided. Printing unsigned transaction for later signing:")
		outputTransactionBlob(&txe)
	}
}



func enterSourceAccount() (acc *stellarwallet.Account, src string, tx *build.TransactionBuilder) {
	for {
		acc = selectSeedAccount("Select Source Account:", true)

		if acc != nil {
			src = acc.PublicKey()
		} else {
			src = getAddressOrSeed("Source")
		}

		tx = tx_setup(src)

		if tx != nil {
			break
		}

		fmt.Println("Source account does not exist or network problems.")
	}

	return
}

func enterDestinationAccount(prompt string) string {
	acc := selectAnyAccount(prompt, true)

	if acc != nil {
		return acc.PublicKey()
	} else {
		return getAddress(prompt)
	}
}

func enterSigners(acc *stellarwallet.Account, key string, tx *build.TransactionBuilder) (bool, build.TransactionEnvelopeBuilder) {

	if acc != nil {
		unlockWallet()
		key = acc.PrivateKey(&g_walletPassword)
		unlockWalletPassword()
	} 
			
	if key != "" {
		kp := keypair.MustParse(key)
		kpf, ok := kp.(*keypair.Full)
		if ok {
			g_signers = append(g_signers, kpf.Seed())
		}
	}

	cnt := readSignersFromFile()

	if cnt == 0 {
		readSigners()
	}

	defer clearSigners()

	return tx_sign(tx)
}


func enterNativePayment(tx *build.TransactionBuilder) {

	dst :=      enterDestinationAccount("Destination")
	amount :=   getPayment("Amount")

	tx_payment(tx, dst, amount)
}	

func enterCreateAccount(tx *build.TransactionBuilder) {

	dst :=      enterDestinationAccount("Destination (new account)")
	amount :=   getPayment("Amount")

	tx_createAccount(tx, dst, amount)
}	

func enterInflationDestination(tx *build.TransactionBuilder) {

	dst :=      enterDestinationAccount("Inflation Destination")

	tx_inflationDestination(tx, dst)
}	


func enterMemo(tx *build.TransactionBuilder) {
	fmt.Println("Select Memo Type:")
	memoTypeMenu := []MenuEntry{
		{ "none", "No Memo", true },
		{ "text", "Text Memo", true},
		{ "id", "ID Memo ", true},
		{ "hash", "Hash Memo", true},
		{ "rethash", "Return Hash Memo", true} }

	choice := runMenu(memoTypeMenu, false)

	switch choice {
	case "none":
		return

	case "text":
		memoTxt := getMemoText("Memo Text")
		tx_memoText(tx, memoTxt)


	case "id":
		memoId := getMemoID("Memo ID")
		tx_memoID(tx, memoId)

	case "hash":
		memoHash := getMemoHash("Memo Hash")
		tx_memoHash(tx, memoHash)

	case "rethash":
		memoHash := getMemoHash("Memo Return Hash")
		tx_memoRetHash(tx, memoHash)

	}
}

func outputTransactionBlob( txe *build.TransactionEnvelopeBuilder) {
	txeB64, err := txe.Base64()
	if err != nil {
		panic(err)
	}

	fmt.Println(txeB64)

	hash, err := network.HashTransaction( &txe.E.Tx, g_network.Passphrase)

	if err != nil {
		panic(err)
	}

	date := time.Now().Format(time.RFC3339)

	var prefix string
	if len(txe.E.Signatures) == 0 {
		prefix = "tx"
	} else {
		prefix = "txs"
	}

	fileName := fmt.Sprintf("%s_%s_%s.txt", prefix, date, hex.EncodeToString(hash[:])[0:8])

	err = writeTransactionBlob(txeB64, txe.E, fileName)

	if err != nil {
		fmt.Printf("Failed to write transaction blob to file \"%s\": %s\n", fileName, err.Error())
	} else {
		fmt.Printf("Transaction blob written to file: %s\n", fileName)
	}

}

func sign_transaction() {
	var tx_s string
	var err error

	if g_txIn != "" {
		fmt.Printf("Reading transaction blob from file: %s\n", g_txIn)
		tx_s, err = readTransactionBlob(g_txIn)
		if err != nil {
			fmt.Printf("Failed to open file \"%s\": %s\n", g_txIn, err.Error())
			return
		}
	} else {
		tx_s = readLine("Transaction blob")
	}
		
	txe_xdr := &xdr.TransactionEnvelope{ }

	txe_xdr.Scan(tx_s)

	if txe_xdr.Tx.SourceAccount.Ed25519  == nil {
		fmt.Printf("Invalid transaction blob: %s\n")
		return
	}

	fmt.Println("\nTransaction details:")
	print_transaction( txe_xdr, "", os.Stdout )

	tx, err := build.Transaction(g_network)
	if err != nil {
		panic(err)
	}
		
	tx.TX = &txe_xdr.Tx

	_, txe := enterSigners(nil, "", tx)

	fmt.Println("\nSigned transaction blob:")	
	outputTransactionBlob(&txe)
	
}

func submit_transaction() {
	var tx_s string
	var err error

	if g_txIn != "" {
		fmt.Printf("Reading transaction blob from file: %s\n", g_txIn)
		tx_s, err = readTransactionBlob(g_txIn)
		if err != nil {
			fmt.Printf("Failed to open file \"%s\": %s\n", g_txIn, err.Error())
			return
		}
	} else {
		tx_s = readLine("Transaction blob")
	}
		
	txe_xdr := &xdr.TransactionEnvelope{ }

	txe_xdr.Scan(tx_s)
	if txe_xdr.Tx.SourceAccount.Ed25519 == nil {
		fmt.Printf("Invalid transaction blob: %s\n")
		return
	}

	fmt.Println("\nTransaction details:")
	print_transaction( txe_xdr, "", os.Stdout )

	if len(txe_xdr.Signatures) == 0 {
		fmt.Printf("\nTransaction is not signed - cannot submit.\n")
		return
	}

	if getOk("Submit transaction") {
		tx_transmit_blob(tx_s)
	}
}	

func transfer_xlm() {
	acc, src, tx := enterSourceAccount()

	enterNativePayment(tx)

	enterMemo(tx)

	transactionFinalize(acc, src, tx)
}

func createAccount() {
	acc, src, tx := enterSourceAccount()

	enterCreateAccount(tx)

	enterMemo(tx)

	transactionFinalize(acc, src, tx)
}

func setInflationDestination() {
	acc, src, tx := enterSourceAccount()

	enterInflationDestination(tx)

	enterMemo(tx)

	transactionFinalize(acc, src, tx)

}

func generateVanityAddress() {

	for {
		var pos = 0
		pattern := readLine("Vanity Address Pattern")

		if pattern != "" {
			plen := len(pattern)
			if plen > 5 {
				if !getOk("It will take a very long time to find patterns with more than 5 characters. Continue anyway?") {
					continue
				}
			}

			pos = getInteger("Position of pattern within address (positive = from left, negative = from right, 0 = no restriction)")
		}
			
		newKeyPair(pattern, pos)
		break
	}
}

func showAccountInfo() {
	var adr string

	acc := selectAnyAccount("Select account", true)
	
	if acc != nil {
		adr = acc.PublicKey()
	} else {
		adr = getAddress("Account")
	}

	accountInfo(adr)
}


func parseCommandLine() {
	flag.BoolVar( &g_testnet, "testnet", false, "switch to testnet")
	flag.StringVar( &g_txIn, "tx-in", "", "path to file containing a transaction blob")
	flag.StringVar( &g_txOut, "tx-out", "", "path to file o which a transaction blob is written")
	flag.StringVar( &g_signersFile, "signers", "", "path to file containing secrect keys for signing transactions")
	flag.StringVar( &g_horizonUrl, "horizon-url", "", "URL to Stellar Horizon server")
	flag.StringVar( &g_walletPath, "wallet-path", "wallet.dat", "wallet file name")
	flag.BoolVar( &g_noWallet, "no-wallet", false, "Disable wallet")
	flag.Parse()
}

func showTransactions() {
	var adr string

	acc := selectAnyAccount("Select account", true)
	
	if acc != nil {
		adr = acc.PublicKey()
	} else {
		adr = getAddress("Account")
	}

	fmt.Printf("\nTransactions for account %s:\n", adr)

	var pagingToken string

	for {
		txs, pagingTokenOut, err :=  getAccountTransactions(adr, 10, pagingToken)
		pagingToken = pagingTokenOut

		if err != nil {
			printHorizonError("load transactions", err)
			return
		}


		for i, _ := range txs {
			tx := &txs[i]
			fmt.Printf("\n%s %s:\n", tx.LedgerCloseTime.Format(time.RFC3339), tx.Hash )
			txe := &xdr.TransactionEnvelope{ }
			
			txe.Scan(tx.EnvelopeXdr)
			
			if txe.Tx.SourceAccount.Ed25519  != nil {
				pretty_print_transaction(txe, adr)
			}
		}

		
		if pagingToken == "" || !getOk("\nShow more transactions") {
			break
		}
	}
				

}

func addTrustLine() {
	acc, src, tx := enterSourceAccount()

	asset := enterAsset("")

	tx_addTrustLine(tx, asset.toHorizonAsset())
	
	enterMemo(tx)

	transactionFinalize(acc, src, tx)
}

func createOrder() {
	acc, src, tx := enterSourceAccount()
	
	selling := enterAsset("Selling")
	buying := enterAsset("Buying")

	price := getPrice("Price")
	amount := getAmount("Amount")

	tx_addOrder(tx, selling, buying, price, amount, 0)
	
	enterMemo(tx)

	transactionFinalize(acc, src, tx)
}



func transaction() {
	menu := []MenuEntryCB{
		{ transfer_xlm, "Transfer Native XLM", true},
		{ createAccount, "Create New Account", true},
		{ addTrustLine, "Create Trust Line", true},
		{ createOrder, "Create Order", true},
		{ setInflationDestination, "Set Inflation Destination", true}}

	
	fmt.Println("TRANSACTION: Select Action:")
	
	runCallbackMenu(menu, "TRANSACTION", false)

}

func lookupFederation() {
	fmt.Println("\nLookup Federation Address:\n")
	adr := getFederationAddress("Enter Federation Address")
	id, memoType, memo := federationLookup(adr)
	if id != "" {
		fmt.Println("Account ID: ", id)
		if memoType != "" {
			fmt.Println("Memo Type : ", memoType)
			fmt.Println("Memo      : ", memo)
		}
	} else {
		fmt.Println("Not found!")
	}	
}

func fundAccount() {
	fmt.Println("\nFund Account")
	adr := getAddress("Account")
	getFund(adr)
	accountInfo(adr)
}


func enterAsset(prompt string) *Asset {
	a, native := selectAsset(prompt, true, true)

	if native {
		return newNativeAsset()
	} else if a != nil {
		return newAssetFrom(a)
	}

	code, issuer := getAsset(prompt)
	return newAsset(issuer, code)
}

func enterTradingPair(prompt string) (asset1, asset2 *Asset) {
	tp := selectTradingPair(prompt, true)

	if tp != nil {
		asset1 = newAssetFrom(tp.Asset1())
		asset2 = newAssetFrom(tp.Asset2())
	} else {
		asset1 = enterAsset(prompt + " Asset 1")
		asset2 = enterAsset(prompt + " Asset 2")
	}

	return
}

func orderBook() {
	fmt.Println("\nShow Order Book")

	selling, buying := enterTradingPair("Trading Pair")

	printOrderBook(selling, buying, 20)
}
	
	

func accountOffers() {
	fmt.Println("Show Account Offers")

	var adr string

	acc := selectAnyAccount("Select account", true)
	
	if acc != nil {
		adr = acc.PublicKey()
	} else {
		adr = getAddress("Account")
	}

	fmt.Printf("\nOffers for %s:\n", adr)

	offers := getOffers(adr, nil, nil)

	if len(offers) > 0 {
		printOffers(offers)
	} else {
		fmt.Println("no offers")
	}
}

func mainMenu() {
	menu := []MenuEntryCB{
		{ walletMenu, "Wallet Menu", g_wallet != nil },
		{ showBalances, "Balances", g_wallet != nil },
		{ showAccountInfo, "Account Info", true },
		{ accountOffers, "Show Account Offers", true},
		{ orderBook, "Show Order Book", true},
		{ trade, "Trading", true},
		{ showTransactions, "Show Account Transactions", true},
		{ transaction, "Primitive Transactions", true},
		{ lookupFederation, "Federation Lookup", true},
		{ generateVanityAddress,  "Generate New Address", true},
		{ sign_transaction,   "Sign Transaction", true},
		{ submit_transaction, "Submit Signed Transaction", true},
		{ fundAccount,  "Fund Account (test network only)", g_testnet} }
	

	runCallbackMenu(menu, "MAIN", true)
}

func walletPasswordResetDaemon() {

	for {
		time.Sleep(time.Second)

		g_walletPasswordLockMutex.Lock()
		if g_walletPasswordLock == 0 {
			if g_walletPassword != "" {
				if time.Since(g_walletPasswordUnlockTime) >= time.Duration(g_walletPasswordLockDuration) * time.Second {
					fmt.Println("Cleared wallet password.")
					stellarwallet.EraseString(&g_walletPassword)
					g_walletPassword = ""
				}
			}

		}

		g_walletPasswordLockMutex.Unlock()
	}


}	

func main() {
	fmt.Printf("stellar-cli version %s (git hash %s)\n\n", g_version, g_gitHash)

	parseCommandLine()

	setupNetwork()

	if !g_noWallet {
		go walletPasswordResetDaemon()
		openOrCreateWallet()
	}

	if flag.Arg(0) != "" {
		kp, err := keypair.Parse(flag.Arg(0))
		if err != nil {
			fmt.Println("ERROR: Invalid Address: ", os.Args[1])
			return
		}
		accountInfo(kp.Address())
		return
	}

	mainMenu()

}


