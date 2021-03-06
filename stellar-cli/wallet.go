package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"io/ioutil"

	"github.com/mua69/stellarwallet"
)

func checkWalletFile() bool {
	info, err := os.Stat(g_walletPath)

	if err != nil {
		return false
	}

	if info.IsDir() {
		return false
	}

	return true

}

func loadWallet() bool {
	data, err := ioutil.ReadFile(g_walletPath)

	if err != nil {
		fmt.Printf("Failed to open wallet file \"%s\": %s\n", g_walletPath, err.Error())
		return false
	}

	g_wallet, err = stellarwallet.ImportBase64(string(data))

	if err != nil {
		fmt.Printf("Failed to parse wallet: %s\n", err.Error())
		return false
	}

	return true
}

func saveWallet() bool {
	data := g_wallet.ExportBase64()

	if data == "" {
		panic("Export wallet failed")
	}

	err := ioutil.WriteFile(g_walletPath, []byte(data), 0600)

	if err != nil {
		fmt.Printf("Failed to write wallet file \"%s\": %s\n", g_walletPath, err.Error())
		return false
	}

	return true
}

func openOrCreateWallet() {
	if checkWalletFile() {
		if loadWallet() {
			fmt.Printf("Loaded Wallet: %s\n", g_walletPath)

			fmt.Println("Enter wallet password to perform integrity check or hit ENTER to skip.")
			if unlockWallet(true) {
				if !g_wallet.CheckIntegrity(&g_walletPassword) {
					fmt.Printf("ATTENTION: Wallet integrity check failed!\n")
				} else {
					fmt.Printf("Wallet integrity check passed.\n")
				}
				unlockWalletPassword()
			} else {
				fmt.Println("Wallet integrity check skipped.")
			}


		} else {
			fmt.Printf("Failed to load wallet.\n")
		}
	} else {

		fmt.Println("Select Wallet Action:")
		walletMenu := []MenuEntry{
			{ "new", "Create New Wallet", true },
			{ "recover", "Recover Wallet With Mnemonic Words", true},
			{ "no", "Continue Without Wallet", true}}
		
		choice := runMenu(walletMenu, false)
		
		switch choice {
		case "no":
			return
			
		case "new":
			createNewWallet()
			

		case "recover":
			recoverWallet()
		}
		
	}

	fmt.Println()
}

func createNewWallet() {
	var pw, wpw string

	fmt.Println("Creating new wallet...")
	getPasswordWithConfirmation("Wallet Password", true, &pw)

	g_wallet = stellarwallet.NewWallet(stellarwallet.WalletFlagSignAll, &pw)

	words := g_wallet.Bip39Mnemonic(&pw)

	fmt.Println("Mnemonic word list required to recover the wallet. Please copy and store in a safe place:")
	
	var table [][]string

	for i := 0; i < 4; i++ {
		table = appendTableLine(table, words[6*i], words[6*i+1], words[6*i+2], words[6*i+3], words[6*i+4], words[6*i+5]) 
	}

	printTable(table, 6, "  ")

	fmt.Println()
	fmt.Println("You may enter an optional mnemonic password. If provided, the password is required to recover the wallet with the mnemonic word list.")

	getPasswordWithConfirmation("Mnemonic Password", false, &wpw)

	if !g_wallet.GenerateBip39Seed(&pw, &wpw) {
		panic("Generation of wallet seed failed")
	}

	if g_wallet.GenerateAccount(&pw) == nil {
		panic("Failed to generate account")
	}

	fmt.Println("New wallet was successfully created.")
	

	if saveWallet() {
		fmt.Printf("Wallet saved to: %s\n", g_walletPath)
	} else {
		fmt.Println("Failed to save wallet.")
	}

	stellarwallet.EraseString(&pw)
}

func splitString(s string) []string {
	var res = make([]string, 0, 10)

	for {
		w := strings.SplitN(s, " ", 2)
		if w == nil || len(w) == 0 {
			break
		}

		res = append(res, w[0])

		if len(w) == 1 {
			break
		}

		s = strings.TrimSpace(w[1])
	}

	return res
}

func getWordList() []string {
	result := make([]string, 24)

	for w := 0; w < 24 ; {
		
		s := readLine(fmt.Sprintf("Enter Word %d", w+1))

		words:= splitString(s)

		for i := range words {
			if w < 24 {
				if !stellarwallet.CheckMnemonicWord(words[i]) {
					fmt.Printf("Invalid mnemonic word: \"%s\"\n", words[i])
					break
				} else {
					result[w] = words[i]
					w++
				}
			}
		}

	}

	return result
}

func fundedCheck(adr string) bool {
	acc, _ := loadAccount(adr)

	if acc != nil {
		return true
	}

	return false
}

func recoverWallet() {
	var pw, wpw string


	fmt.Println("Recovering wallet from mnemonic word list...")
	fmt.Println()
	fmt.Println("Define new  password for the wallet - this is not the mnemonic password.")
	
	getPasswordWithConfirmation("Wallet Password", true, &pw)

	for g_wallet == nil {
		fmt.Println("Enter 24 mnemonic words:")
		words := getWordList()

		getPasswordWithConfirmation("Mnemonic Password", false, &wpw)

		g_wallet = stellarwallet.NewWalletFromMnemonic(stellarwallet.WalletFlagSignAll, &pw, words, &wpw)

		if g_wallet == nil {
			fmt.Println("Invalid mnemonic words.")
		}
	}
	
	
	if g_wallet.GenerateAccount(&pw) == nil {
		panic("Failed to generate account")
	}

	if g_online {
		g_wallet.RecoverAccounts(&pw, 5, fundedCheck)
	} else {
		fmt.Println("OFFLINE: Cannot automatically recover accounts.")
	}

	fmt.Println("Wallet was successfully recovered.")
	

	if saveWallet() {
		fmt.Printf("Wallet saved to: %s\n", g_walletPath)
	} else {
		fmt.Println("Failed to save wallet.")
	}

	stellarwallet.EraseString(&pw)
	
}

func walletMenu() {
	

	walletMenu := []MenuEntryCB{
		{ listWallet, "List Accounts", true },
		{ listAssets, "List Assets", true },
		{ listTradingPairs, "List Trading Pairs", true },
		{ accountMenu, "Manage Accounts", true },
		{ assetMenu, "Manage Assets", true },
		{ tradingPairMenu, "Manage Trading Pairs", true },
		{ changePassword, "Change Password", true}}

		
	runCallbackMenu(walletMenu, "WALLET: Select Action", true)
}

func accountMenu() {

	menu := []MenuEntryCB{
		{ generateAccount, "Generate New Account", true},
		{ addRandomAccount, "Add Random Account", true},
		{ addWatchingAccount, "Add Watching Account", true},
		{ addAddressBookAccount, "Add Address Book Account", true},
		{ changeAccountDescription, "Change Account Description", true},
		{ changeAccountMemo, "Change Account Memo Text/ID", true},
		{ deleteAccount, "Delete Account", true}}
		
	runCallbackMenu(menu, "ACCOUNT: Select Action", false)

}

func assetMenu() {

	menu := []MenuEntryCB{
		{ addAsset, "Add New Asset", true},
		{ changeAssetDescription, "Change Asset Description", true},
		{ deleteAsset, "Delete Asset", true}}
		
	runCallbackMenu(menu, "ASSET: Select Action", false)

}


func accountTypeToString(a *stellarwallet.Account) string {
	switch a.Type() {
	case stellarwallet.AccountTypeSEP0005:
		return "G"
	case stellarwallet.AccountTypeRandom:
		return "R"
	case stellarwallet.AccountTypeWatching:
		return "W"
	case stellarwallet.AccountTypeAddressBook:
		return "A"
	}

	return "U"
}

		
	

func listWallet() {
	accounts := g_wallet.Accounts()

//	var table [][]string	

	fmt.Println("Accounts:")
	for _, a := range accounts {
		fmt.Printf("%s: %s %s\n", accountTypeToString(a), a.PublicKey(), a.Description())
		if a.MemoText() != "" {
			fmt.Printf("   Memo Text: %s\n", a.MemoText())
		}
		memoIdSet, memoId := a.MemoId()
		if memoIdSet {
			fmt.Printf("   Memo ID: %d\n", memoId)
		}
		if g_online {
			ai, err := loadAccount(a.PublicKey())
			if err != nil {
				printHorizonError("load account", err)
			} else {
				if ai != nil {
					nb, _ := ai.GetNativeBalance()
					fmt.Printf("   XLM: %s\n", nb)

				} else {
					fmt.Printf("   Not funded\n")
				}
			}
		}		
	}
	
//	printTable(table, 3, " ")
	fmt.Println()
}

func listAssets() {
	assets := g_wallet.Assets()

	fmt.Println("\nAssets:")
	for _, a := range assets {
		fmt.Printf("%s/%s %s\n", a.AssetId(), a.Issuer(), a.Description()) 
	}
}

func lockWalletPassword() {
	g_walletPasswordLockMutex.Lock()
	g_walletPasswordLock++
	g_walletPasswordLockMutex.Unlock()
}

func unlockWalletPassword() {
	g_walletPasswordLockMutex.Lock()
	if g_walletPasswordLock > 0 {
		g_walletPasswordLock--
	}
	g_walletPasswordLockMutex.Unlock()
}

// prompts for wallet password if required
// locks wallet password to avoid reset by time out
//  unlockWalletPassword() must called after wallet password related processing is done
func unlockWallet(optional bool) bool {
	lockWalletPassword()

	if g_walletPassword != "" {
		return true
	}

	for {
		getPassword("Wallet Password", !optional, &g_walletPassword)
		if g_walletPassword == "" {
			unlockWalletPassword()
			return false
		}
		if g_wallet.CheckPassword(&g_walletPassword) {
			break
		} else {
			fmt.Println("Invalid password.")
		}
	}

	g_walletPasswordUnlockTime = time.Now()

	return true
}

func lockWallet() {
	lockWalletPassword()
	g_walletPassword = ""
	unlockWalletPassword()
}

func enterAccountDescription(a *stellarwallet.Account) {
	unlockWallet(false)
	defer unlockWalletPassword()

	for {
		desc := readLine("Account description")
		err := a.SetDescription(desc, &g_walletPassword)
		if err != nil {
			fmt.Printf("Invalid description: %s\n", err.Error())
		} else {
			break
		}
	}

}

func enterAssetDescription(a *stellarwallet.Asset) {
	unlockWallet(false)
	defer unlockWalletPassword()

	for {
		desc := readLine("Asset description")
		err := a.SetDescription(desc, &g_walletPassword)
		if err != nil {
			fmt.Printf("Invalid description: %s\n", err.Error())
		} else {
			break
		}
	}
}

func enterAccountMemoText(a *stellarwallet.Account) {
	unlockWallet(false)
	defer unlockWalletPassword()

	for {
		memo := readLine("Memo Text")
		err := a.SetMemoText(memo, &g_walletPassword)
		if err != nil {
			fmt.Printf("Invalid memo text: %s\n", err.Error())
		} else {
			break
		}
	}
}

func enterAccountMemoID(a *stellarwallet.Account) {
	unlockWallet(false)
	defer unlockWalletPassword()

	memo := getMemoID("Memo ID")
	a.SetMemoId(memo, &g_walletPassword)
}


func changePassword() {
	fmt.Println("Changing Wallet Password...")

	unlockWallet(false)
	defer unlockWalletPassword()

	var pw string
	getPasswordWithConfirmation("New Wallet Password", true, &pw)

	if !g_wallet.ChangePassword(&g_walletPassword, &pw) {
		// should never happen
		fmt.Println("Change wallet password failed.")
	} else {
		fmt.Println("Wallet password changed.")
		lockWallet()
		saveWallet()
	}
}


func generateAccount() {
	unlockWallet(false)
	defer unlockWalletPassword()

	a := g_wallet.GenerateAccount(&g_walletPassword)

	if a != nil {
		fmt.Printf("New account: %s\n", a.PublicKey())
		enterAccountDescription(a)
		saveWallet()
	} else {
		fmt.Println("Failed to generate account.")
	}
}

func addRandomAccount() {
	unlockWallet(false)
	defer unlockWalletPassword()

	seed := getSeed("Account", false)

	a := g_wallet.AddRandomAccount(&seed, &g_walletPassword)

	if a != nil {
		fmt.Printf("New account: %s\n", a.PublicKey())
		enterAccountDescription(a)
		saveWallet()
	} else {
		fmt.Println("Failed to add random account.")
	}
}
		
func addWatchingAccount() {
	unlockWallet(false)
	defer unlockWalletPassword()

	acc := getAddress("Account")

	a := g_wallet.AddWatchingAccount(acc, &g_walletPassword)

	if a != nil {
		fmt.Printf("New watching account: %s\n", a.PublicKey())
		enterAccountDescription(a)
		saveWallet()
	} else {
		fmt.Println("Failed to add watching account.")
	}
}

func addAddressBookAccount() {
	unlockWallet(false)
	defer unlockWalletPassword()

	acc := getAddress("Account")

	a := g_wallet.AddAddressBookAccount(acc, &g_walletPassword)

	if a != nil {
		fmt.Printf("New address book account: %s\n", a.PublicKey())
		enterAccountDescription(a)
		saveWallet()
	} else {
		fmt.Println("Failed to add address book account.")
	}
}
		
func selectAccount(prompt string, enterAccountOption bool, accounts []*stellarwallet.Account) *stellarwallet.Account {
	if len(accounts) == 0 {
		return nil
	}

	menu := make([]MenuEntry, 0, len(accounts))

	if enterAccountOption {
		menu = append(menu, MenuEntry{ "enter", "Enter Account", true})
	}

	for _, a := range accounts {
		menu = append(menu, MenuEntry{ a.PublicKey(), accountTypeToString(a) + ": " + a.PublicKey() + " " + a.Description(), true})
	}

	fmt.Println(prompt + ":")
	sel := runMenu(menu, false)

	if sel == "enter" {
		return nil
	}

	for _, a := range accounts {
		if a.PublicKey() == sel {
			return a
		}
	}

	return nil
}

func selectSeedAccount(prompt string, enterAccountOption bool) *stellarwallet.Account {
	if g_wallet != nil {
		return selectAccount(prompt, enterAccountOption, g_wallet.SeedAccounts())
	} else {
		return nil
	}
}

func selectAnyAccount(prompt string, enterAccountOption bool) *stellarwallet.Account {
	if g_wallet != nil {
		accounts := g_wallet.Accounts()
		accounts = append(accounts, g_wallet.AddressBook()...)
		return selectAccount(prompt, enterAccountOption, accounts)
	} else {
		return nil
	}
}

func deleteAccount() {
	a := selectAnyAccount("Select account to delete", false)

	if a == nil { return }

	pubkey := a.PublicKey()
	fmt.Printf("Deleting account %s %s...\n", pubkey, a.Description())
	if getOk("Delete Account") {
		
		if !g_wallet.DeleteAccount(a) {
			fmt.Println("Delete account failed.")
		} else {
			fmt.Printf("Deleted account: %s\n", pubkey)
			saveWallet()
		}
	}
}

func changeAccountDescription() {
	a := selectAnyAccount("Select account to edit", false)

	if a != nil {
		enterAccountDescription(a)
		saveWallet()
	}
}




func changeAccountMemo() {
	a := selectAnyAccount("Select account to edit", false)

	if a != nil {
		menu := []MenuEntryCB{
			{ func () { enterAccountMemoText(a) }, "Set Memo Text", true},
			{ func () { enterAccountMemoID(a) }, "Set Memo ID", true},
			{ func () { a.ClearMemoId(nil) }, "Clear Memo ID", true}}
		
		runCallbackMenu(menu, "EDIT MEMO: Select Action", false)
		saveWallet()
	}
}


func selectAsset(prompt string, enterAssetOption, nativeOption bool) (selectedAsset *stellarwallet.Asset, native bool) {
	if g_wallet == nil {
		return nil, false
	}

	assets := g_wallet.Assets()

	if len(assets) == 0 && !nativeOption {
		return nil, false
	}

	menu := make([]MenuEntry, 0, len(assets)+2)

	type choiceType struct {
		id string
		asset *stellarwallet.Asset
		native bool }
	choices := make([]choiceType, 0, len(assets)+2)

	choice := 1

	if enterAssetOption {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, "Enter Asset", true})
		choices = append(choices, choiceType{s, nil, false})
		choice++
	}

	if nativeOption {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, "XLM", true})
		choices = append(choices, choiceType{s, nil, true})
		choice++
	}

	for _, a := range assets {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, a.AssetId() + "/" + a.Issuer() + " " + a.Description(), true})
		choices = append(choices, choiceType{s, a, false})
		
		choice++
	}

	fmt.Printf("\n%s:\n", prompt)
	sel := runMenu(menu, false)

	for i := range choices {
		if choices[i].id == sel {
			return choices[i].asset, choices[i].native 
		}
	}

	return nil, false
}

func addAsset() {
	fmt.Println("\nAdd new asset...")

	id, issuer := getAsset("New")

	if g_wallet.FindAsset(issuer, id) != nil {
		fmt.Println("Asset already exists.")
		return
	}

	unlockWallet(false)
	defer unlockWalletPassword()

	a := g_wallet.AddAsset(issuer, id, &g_walletPassword)

	if a == nil {
		panic("add asset failed")
	}

	enterAssetDescription(a)

	saveWallet()
}

func deleteAsset() {
	a, _ := selectAsset("Select Asset to Delete", false, false)

	if a != nil {
		fmt.Printf("Deleting Asset %s/%s %s...\n", a.AssetId(), a.Issuer(), a.Description())
		if getOk("Delete Asset") {

			if !g_wallet.DeleteAsset(a) {
				fmt.Println("Delete asset failed.")
			} else {
				fmt.Printf("Deleted asset\n")
				saveWallet()
			}
		}
	}
}


func changeAssetDescription() {
	a, _ := selectAsset("Select Asset to Edit", false, false)

	if a != nil {
		enterAssetDescription(a)
		saveWallet()
	}
}



func tradingPairMenu() {

	menu := []MenuEntryCB{
		{ addTradingPair, "Add New Trading Pair", true},
		{ changeTradingPairDescription, "Change Trading Pair Description", true},
		{ deleteTradingPair, "Delete Trading Pair", true}}
		
	runCallbackMenu(menu, "TRADING PAIR: Select Action", false)
}

func listTradingPairs() {
	fmt.Println("\nTrading Pairs:")
	tps := g_wallet.TradingPairs()

	for _, tp := range tps {
		fmt.Printf("%s<->%s %s\n", assetToStringPretty(tp.Asset1()), assetToStringPretty(tp.Asset2()), 
			tp.Description())
	}
}

func enterTradingPairDescription(tp *stellarwallet.TradingPair) {
	unlockWallet(false)
	defer unlockWalletPassword()

	for {
		desc := readLine("Trading Pair description")
		err := tp.SetDescription(desc, &g_walletPassword)
		if err != nil {
			fmt.Printf("Invalid description: %s\n", err.Error())
		} else {
			break
		}
	}
}

func addTradingPair() {
	fmt.Println("\nAdd new trading pair...")
	
	asset1, _ := selectAsset("Asset 1", false, true)
	asset2, _ := selectAsset("Asset 2", false, true)

	tp := g_wallet.FindTradingPair(asset1, asset2)

	if tp != nil {
		fmt.Println("Trading pair already exists")
		return
	}

	unlockWallet(false)
	defer unlockWalletPassword()

	tp = g_wallet.AddTradingPair(asset1, asset2, &g_walletPassword)

	if tp == nil {
		fmt.Println("Invalid asset pair.")
		return
	}

	enterTradingPairDescription(tp)

	saveWallet()

	fmt.Println("New trading pair successfully defined.")
}

func abbreviateIssuer(s string) string {
	return s[:5] + "..." + s[len(s)-5:]
}

func assetToStringPretty(a *stellarwallet.Asset) string {
	if a == nil {
		return "XLM"
	}

	s := a.AssetId() +  "/" + abbreviateIssuer(a.Issuer())

	return s
}


func selectTradingPair(prompt string, enterOption bool) *stellarwallet.TradingPair {
	if g_wallet == nil {
		return nil
	}

	tps := g_wallet.TradingPairs()

	if len(tps) == 0 {
		return nil
	}

	menu := make([]MenuEntry, 0, len(tps)+1)

	type choiceType struct {
		id string
		tp *stellarwallet.TradingPair
		}
	choices := make([]choiceType, 0, len(tps)+1)

	choice := 1

	if enterOption {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, "Enter Trading Pair", true})
		choices = append(choices, choiceType{s, nil})
		choice++
	}

	for _, tp := range tps {
		s := fmt.Sprintf("%d", choice)
		menu = append(menu, MenuEntry{ s, newAssetFrom(tp.Asset1()).StringPretty() + "<->" + newAssetFrom(tp.Asset2()).StringPretty() +
			" " + tp.Description(), true})
		choices = append(choices, choiceType{s, tp})
		
		choice++
	}

	fmt.Printf("\n%s:\n", prompt)
	sel := runMenu(menu, false)

	for i := range choices {
		if choices[i].id == sel {
			return choices[i].tp
		}
	}

	return nil
}

func changeTradingPairDescription() {
	fmt.Println("\nChange trading pair description...")

	tp := selectTradingPair("Trading Pair", false)

	enterTradingPairDescription(tp)

	saveWallet()
}

func deleteTradingPair() {
	fmt.Println("\nDelete trading pair...")

	tp := selectTradingPair("Trading Pair to delete", false)

	if !g_wallet.DeleteTradingPair(tp) {
		fmt.Println("Delete trading pair failed.")
	} else {
		fmt.Printf("Deleted trading pair\n")
		saveWallet()
	}
}

