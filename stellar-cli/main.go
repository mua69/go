package main

import (
	"fmt"
	"flag"
	"strings"
	"net/http"
	"io/ioutil"
	"os"
	"bufio"
	"encoding/hex"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/build"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/clients/stellartoml"
	"github.com/stellar/go/clients/federation"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/network"
)


var (
	NW_PASS = build.TestNetwork
	CLIENT = horizon.DefaultTestNetClient
	g_version = "1.0"
	g_gitHash = "undefined"

	//flags
	g_tx_in = ""
	g_tx_out = ""
	g_signers = ""
	g_testnet = false
)

func setupNetwork() {
	if g_testnet {
		NW_PASS = build.TestNetwork
		CLIENT = horizon.DefaultTestNetClient
	} else {
		NW_PASS = build.PublicNetwork 	
		CLIENT = &horizon.Client{
			URL:  "https://horizon.stellarport.earth/",
			HTTP: http.DefaultClient,
		}
	}
			
	fmt.Println("Using Network       :", NW_PASS)
	fmt.Println("Using Horizon Server:", CLIENT.URL)
	fmt.Println()
}


func newKeypair() {

	kp, err := keypair.Random()
        index := 0
	for {
                index = strings.Index(kp.Address(), "DMUA")
		if index > 0 && index < 5 { break }
		kp, err = keypair.Random()
	}
	fmt.Println("Address:", kp.Address())
	fmt.Println("Seed   :", kp.Seed())
	if err != nil {
		panic(err)
	}

}

func getFund(adr string) {
    // pair is the pair that was generated from previous example, or create a pair based on 
    // existing keys.

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
		Horizon:     CLIENT,
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



func accountInfo(adr string) {
	var table [][]string

	acc, err := CLIENT.LoadAccount(adr)
	if err != nil {
		if herr, ok := err.(*horizon.Error); ok {
			if herr.Problem.Title == "Resource Missing" {
				fmt.Println("Account does not exist: ", adr)
			} else {
				panic(herr)
			}
		} else { panic(err) }
	} else {
		table = appendTableLine(table, "Address", adr)
		table = appendTableLine(table, "Balance (XLM)", acc.GetNativeBalance())
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

		table = appendTableLine(table, "Account Flags", strings.Join(flags, " "))

		table = appendTableLine(table, "Low Threshold", fmt.Sprintf("%d", acc.Thresholds.LowThreshold))
		table = appendTableLine(table, "Med Threshold", fmt.Sprintf("%d", acc.Thresholds.MedThreshold))
		table = appendTableLine(table, "High Threshold", fmt.Sprintf("%d", acc.Thresholds.HighThreshold))
		
		for _, signer := range acc.Signers {
			table = appendTableLine(table, "Signer", fmt.Sprintf("%s Weight:%d Key:%s Type:%s", signer.PublicKey,
				signer.Weight, signer.Key, signer.Type))
		}
			
		printTable(table, 2, ": ")
	}
}

func transfer_xlm() {
	var table [][]string

	src :=      getAddressOrSeed("Source")
	dst :=      getAddress("Destination")
	amount :=   getPayment("Amount")
	memoTxt := getMemoText("Memo Text (optional)")
	//memoID :=    getMemoID("Memo ID (optional)")
	
	fmt.Println("\nTransaction summary:")
	
	table = appendTableLine(table, "Source Address", keypair.MustParse(src).Address())
	table = appendTableLine(table, "Destination Address", dst)
	table = appendTableLine(table, "Amount (native XLM)", amount)
	table = appendTableLine(table, "Memo Text", "<" + memoTxt + ">")
	//fmt.Println("Memo ID            :", memoID)
	printTable(table, 2, ": ")
	

	fmt.Println()

	tx := tx_setup(src)
	tx_payment(tx, dst, amount)
	if memoTxt != "" {
		tx_memoText(tx, memoTxt)
	}
	tx_finalize(tx)

	fmt.Println("Fee:", tx.TX.Fee)	

	kp := keypair.MustParse(src)
	kpf, ok := kp.(*keypair.Full)
	if ok {
		if getOk("Transmit transaction") {
			txe := tx_sign(tx, kpf.Seed())
			fmt.Println("Fee:", txe.E.Tx.Fee)	
			tx_transmit(txe)
		} else {
			fmt.Println("Transaction aborted.")
		}
	} else {
		fmt.Println("No signing key provided. Printing unsigned transaction for later signing:")
		txe := tx.Sign()
		outputTransactionBlob(&txe)
	}
}



func outputTransactionBlob( txe *build.TransactionEnvelopeBuilder) {
	txeB64, err := txe.Base64()
	if err != nil {
		panic(err)
	}

	fmt.Println(txeB64)

	hash, err := network.HashTransaction( &txe.E.Tx, NW_PASS.Passphrase)

	if err != nil {
		panic(err)
	}

	fileName := fmt.Sprintf("tx_%s.txt", hex.EncodeToString(hash[:])[0:8])

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

	if g_tx_in != "" {
		fmt.Printf("Reading transaction blob from file: %s\n", g_tx_in)
		tx_s, err = readTransactionBlob(g_tx_in)
		if err != nil {
			fmt.Printf("Failed to open file \"%s\": %s\n", g_tx_in, err.Error())
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

	tx := build.Transaction(
		NW_PASS)
	tx.TX = &txe_xdr.Tx

	key := getSeed("Signing key")
	txe := tx.Sign(key)

	fmt.Println("\nSigned transaction blob:")	
	outputTransactionBlob(&txe)
	
}

func submit_transaction() {
	var tx_s string
	var err error

	if g_tx_in != "" {
		fmt.Printf("Reading transaction blob from file: %s\n", g_tx_in)
		tx_s, err = readTransactionBlob(g_tx_in)
		if err != nil {
			fmt.Printf("Failed to open file \"%s\": %s\n", g_tx_in, err.Error())
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

func parseCommandLine() {
	flag.BoolVar( &g_testnet, "testnet", false, "switch to testnet")
	flag.StringVar( &g_tx_in, "tx-in", "", "path to file containing a transaction blob")
	flag.StringVar( &g_tx_out, "tx-out", "", "path to file o which a transaction blob is written")
	flag.StringVar( &g_signers, "signers", "", "path to file containing secrect keys for signing transactions")
	flag.Parse()
}

	
func main() {
	fmt.Printf("stellar-cli version %s (git hash %s)\n\n", g_version, g_gitHash)

	parseCommandLine()

	setupNetwork()


	if flag.Arg(0) != "" {
		kp, err := keypair.Parse(flag.Arg(0))
		if err != nil {
			fmt.Println("ERROR: Invalid Address: ", os.Args[1])
			return
		}
		accountInfo(kp.Address())
		return
	}

	in := bufio.NewReader(os.Stdin)
	action := ""

	fmt.Println("Select Action:\n")
	fmt.Println("1  Print Account Info")
	fmt.Println("2  Transfer Native XLM")
	fmt.Println("3  Create New Account")
	fmt.Println("4  Set Inflation Destination")
	fmt.Println("5  Fund Account (testnet)")
	fmt.Println("6  Federation Lookup")
	fmt.Println("7  Generate New Address")
	fmt.Println("8  Sign Transaction")
	fmt.Println("9  Submit Signed Transaction")
	
	fmt.Println("q  Quit")


	for ; action == ""; {
		fmt.Printf("\n--> ")
		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimRight(input, "\r\n")
		
		switch input {
		case "1":
			action = "info"
		case "2":
			action = "tran_xlm"
		case "3":
			action = "create_acc"
		case "4":
			action = "infl_dst"
		case "5":
			action = "fund"
		case "6":
			action = "fed_lookup"
		case "7":
			action = "new_adr"
		case "8":
			action = "sign_tx"
		case "9":
			action = "submit_tx"
		case "q":
			fmt.Println("Quit.")
			return
		default:
			fmt.Printf("Invalid input: %s\n", input)
		}
	}

	switch action {
	case "info":
		fmt.Println("\n--> Print Account Info\n")
		adr := getAddress("Account")
		accountInfo(adr)

	case "fed_lookup":
		fmt.Println("\n--> Lookup Federation Address\n")
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

	case "tran_xlm":
		fmt.Println("\n--> Transfer XLM\n")
		transfer_xlm()

	case "create_acc":
		fmt.Println("\n--> Create Account\n")
		src :=         getSeed("Source")
		dst :=      getAddress("Destination")
		amount :=   getPayment("Amount")


		fmt.Println("\nTransaction summary:")
		fmt.Println("Source Address     :", keypair.MustParse(src).Address())
		fmt.Println("Destination Address:", dst)
		fmt.Println("Amount (native XLM):", amount)
		fmt.Println()

		if getOk("Transmit transaction") {
			createAccount(src, dst, amount);
		} else {
			fmt.Println("Transaction aborted.")
		}

	case "infl_dst":
		fmt.Println("\n--> Set Inflation Destination\n")
		src := getSeed("Source")
		dst := getAddress("Inflation Destination")
		prompt := fmt.Sprintf("Set inflation destination of %s to %s ", keypair.MustParse(src).Address(), dst)
		if getOk(prompt) {
			setInflationDestination(src, dst)
		} else {
			fmt.Println("Transaction aborted.")
		}
	case "fund":
		fmt.Println("\n--> Fund Account\n")
		adr := getAddress("Account")
		getFund(adr)
		accountInfo(adr)
	case "new_adr":
		fmt.Println("\n--> Create New Public/Private Key\n")
		newKeypair()
	case "sign_tx":
		fmt.Println("\n--> Sign Transaction\n")
		sign_transaction()
	case "submit_tx":
		fmt.Println("\n--> Submit Signed Transaction\n")
		submit_transaction()

	}


/*
    cursor := horizon.Cursor("now")

    ctx, cancel := context.WithCancel(context.Background())

    go func() {
      // Stop streaming after 60 seconds.
      time.Sleep(60 * time.Second)
      cancel()
    }()

    err = client.StreamLedgers(ctx, &cursor, func(l horizon.Ledger) {
      fmt.Println(l.Sequence)
    })

    if err != nil {
      fmt.Println(err)
    }
*/
}


