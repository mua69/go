package main

import (
	"fmt"
	"strings"
	//    "time"
	"net/http"
	"io/ioutil"
	"os"
	"bufio"
	//"log"
	//    "golang.org/x/net/context"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/stellartoml"
	"github.com/stellar/go/clients/federation"
	"github.com/stellar/go/clients/horizon"
)


var (
	NW_PASS = build.TestNetwork
	CLIENT = horizon.DefaultTestNetClient

)

func setTestNetwork() {
	NW_PASS = build.TestNetwork
	CLIENT = horizon.DefaultTestNetClient
}

func setPublicNetwork() {
	NW_PASS = build.PublicNetwork 	
	CLIENT = &horizon.Client{
		URL:  "https://horizon.stellarport.earth/",
		HTTP: http.DefaultClient,
	}
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

func printTransactionResults( txr horizon.TransactionSuccess) {
	fmt.Println("Transaction posted in ledger: ", txr.Ledger)
	fmt.Println("Transaction hash            : ", txr.Hash)
}

func createAccount(src string, dst string, amount string) {

	tx := build.Transaction(
		build.SourceAccount{src},
		build.AutoSequence{CLIENT},
		NW_PASS,
		build.CreateAccount(
			build.Destination{dst},
			build.NativeAmount{amount}),
	)

	txe := tx.Sign(src)
	txeB64, _ := txe.Base64()

	fmt.Printf("tx base64: %s\n", txeB64)

	resp, err := CLIENT.SubmitTransaction(txeB64)
	if err != nil {
		panic(err)
	}

	printTransactionResults(resp)
}

func setInflationDestination(src, dst string) {
	tx := build.Transaction(
		build.SourceAccount{src},
		build.AutoSequence{CLIENT},
		NW_PASS,
		build.SetOptions(build.InflationDest(dst)),
	)
    
	txe := tx.Sign(src)
	txeB64, err := txe.Base64()
	
	if err != nil {
		panic(err)
	}

	fmt.Printf("tx base64: %s\n", txeB64)
	
	resp, err := CLIENT.SubmitTransaction(txeB64)
	if err != nil {
		panic(err)
	}

	printTransactionResults(resp)
}

func tx_setup( src string ) (tx *build.TransactionBuilder) {
	tx = build.Transaction(
		build.SourceAccount{src},
		build.AutoSequence{CLIENT},
		NW_PASS)
	return
}

func tx_createAccount(tx *build.TransactionBuilder, dst string, amount string) {

	tx.Mutate(
		build.CreateAccount(
			build.Destination{dst},
			build.NativeAmount{amount}))
}

func tx_payment( tx *build.TransactionBuilder, dst string, amount string) {
	tx.Mutate(build.Payment(
		build.Destination{dst},
		build.NativeAmount{amount}))
}

func tx_InflationDestination( tx *build.TransactionBuilder, dst string) {
	tx.Mutate(
		build.SetOptions(build.InflationDest(dst)))
}

func tx_memoText( tx *build.TransactionBuilder, memoText string ) {
	tx.Mutate(build.MemoText{memoText})
}



func payment( src string, dst string, amount string, memoText string) {
	var tx *build.TransactionBuilder

	tx = build.Transaction(
		build.SourceAccount{src},
		build.AutoSequence{CLIENT},
		NW_PASS,
		build.Payment(
			build.Destination{dst},
			build.NativeAmount{amount}))

	if memoText != "" {
		tx.Mutate( build.MemoText{memoText} )
	}
    
	txe := tx.Sign(src)
	txeB64, err := txe.Base64()
	
	if err != nil {
		panic(err)
	}

	fmt.Printf("tx base64: %s\n", txeB64)
	
	resp, err := CLIENT.SubmitTransaction(txeB64)
	if err != nil {
		panic(err)
	}

	printTransactionResults(resp)
}

func accountInfo(adr string) {
	acc, err := CLIENT.LoadAccount(adr)
	if err != nil {
		if herr, ok := err.(*horizon.Error); ok {
			if herr.Problem.Title == "Resource Missing" {
				fmt.Println("Account does not exist: ", adr)
			} else { panic(err)
			}
		} else { panic(err) }
	} else {
		fmt.Println("Address:", adr)
		fmt.Println("Balance:", acc.GetNativeBalance())
		fmt.Println("Inflation Destination: ", acc.InflationDestination);
	}
}
	
func main() {
	//setTestNetwork()
	setPublicNetwork()

	if len(os.Args) == 2 {
		kp, err := keypair.Parse(os.Args[1])
		if err != nil {
			fmt.Println("ERROR: Invalid Address: ", os.Args[1])
			return
		}
		accountInfo(kp.Address())
		return
	}

	in := bufio.NewReader(os.Stdin)
	action := ""

	fmt.Println("Select Action:")
	fmt.Println("1  Print Account Info")
	fmt.Println("2  Transfer Native XLM")
	fmt.Println("3  Create New Account")
	fmt.Println("4  Set Inflation Destination")
	fmt.Println("5  Fund Account (testnet)")
	fmt.Println("6  Federation Lookup")
	fmt.Println("7  Generate New Address")

	
	fmt.Println("q  Quit")

	for ; action == ""; {
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
		src :=         getSeed("Source")
		dst :=      getAddress("Destination")
		amount :=   getPayment("Amount")
		memoTxt := getMemoText("Memo Text (optional)")
		//memoID :=    getMemoID("Memo ID (optional)")

		fmt.Println("\nTransaction summary:")
		fmt.Println("Source Address     :", keypair.MustParse(src).Address())
		fmt.Println("Destination Address:", dst)
		fmt.Println("Amount (native XLM):", amount)
		fmt.Printf ("Memo Text          : <%s>\n", memoTxt)
		//fmt.Println("Memo ID            :", memoID)
		fmt.Println()

		if getOk("Transmit transaction") {
			payment(src, dst, amount, memoTxt);
		} else {
			fmt.Println("Transaction aborted.")
		}


	case "create_acc":
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
		newKeypair()
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


