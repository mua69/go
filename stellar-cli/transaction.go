package main

import (
	"os"
	"fmt"
	"io"
	"bufio"
	"strings"
	"strconv"
	"encoding/hex"
	"math/big"
	"github.com/stellar/go/build"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/amount"
)




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

func tx_sign( tx *build.TransactionBuilder, key string) (build.TransactionEnvelopeBuilder) {
	return tx.Sign(key)
}

func tx_finalize( tx *build.TransactionBuilder ) {
	tx.Mutate(build.Defaults{})
}

func tx_transmit( txe build.TransactionEnvelopeBuilder ) {
	txeB64, err := txe.Base64()
	
	if err != nil {
		panic(err)
	}

	tx_transmit_blob(txeB64)
}

func tx_transmit_blob( tx_blob string ) {
	resp, err := CLIENT.SubmitTransaction(tx_blob)
	if err != nil {
		if herr, ok := err.(*horizon.Error); ok {
			fmt.Println(herr.Problem.Title)
			fmt.Println(herr.Problem.Detail)
			fmt.Println(string(herr.Problem.Extras["result_codes"]))
			
			panic(herr)
		} else { panic(err) }
	}

	printTransactionResults(resp)
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

func rawPublicKeyToString( k xdr.AccountId) string {
	var b32 [32]byte = *k.Ed25519

	return strkey.MustEncode(strkey.VersionByteAccountID, b32[:])
}

func paymentOpToString( op *xdr.PaymentOp) string {
	return "DST:" + rawPublicKeyToString(op.Destination) + " AMT:" + op.Asset.String() + " " + amount.String(op.Amount)
}

func createAccountOpToString( op *xdr.CreateAccountOp) string {
	return "DST:" + rawPublicKeyToString(op.Destination) + " AMT:" + amount.String(op.StartingBalance)
}

func setOptionsFlagsToString( f xdr.Uint32 ) string {
	var flags []string;

	d := xdr.AccountFlags(f)

	if d & xdr.AccountFlagsAuthRequiredFlag != 0 {
		flags = append(flags, "AUTH_REQUIRED")
	}

	if d & xdr.AccountFlagsAuthRevocableFlag != 0 {
		flags = append(flags, "AUTH_REVOCABLE")
	}

	if d & xdr.AccountFlagsAuthImmutableFlag != 0 {
		flags = append(flags, "AUTH_IMMUTABLE")
	}

	return strconv.FormatUint(uint64(f), 16) + "(" + strings.Join(flags, ",") + ")"
}

func setOptionsOpToString( op *xdr.SetOptionsOp) string {
	var r []string

	if op.InflationDest != nil {
		r = append(r, "INFLATION_DST:" + rawPublicKeyToString(*op.InflationDest))
	}

	if op.ClearFlags != nil {
		r = append(r, "CLEAR_FLAGS:" + setOptionsFlagsToString(*op.ClearFlags))
	}

	if op.SetFlags != nil {
		r = append(r, "SET_FLAGS:" + setOptionsFlagsToString(*op.SetFlags))
	}

	if op.MasterWeight != nil {
		r = append(r, "MASTER_WEIGHT:" + strconv.FormatUint(uint64(*op.MasterWeight), 10))
	}

	if op.LowThreshold != nil {
		r = append(r, "LOW_THRESHOLD:" + strconv.FormatUint(uint64(*op.LowThreshold), 10))
	}

	if op.MedThreshold != nil {
		r = append(r, "MED_THRESHOLD:" + strconv.FormatUint(uint64(*op.MedThreshold), 10))
	}

	if op.HighThreshold != nil {
		r = append(r, "HIGH_THRESHOLD:" + strconv.FormatUint(uint64(*op.HighThreshold), 10))
	}
	
	if op.HomeDomain != nil {
		r = append(r, "HOME_DOMAIN:" + string(*op.HomeDomain))
	}

	if op.Signer != nil {
		if op.Signer.Weight == 0 {
			r = append(r, "REMOVE_SIGNER:", op.Signer.Key.Address())
		} else {
			r = append(r, "ADD_SIGNER:", op.Signer.Key.Address(), ":", strconv.FormatUint(uint64(op.Signer.Weight), 10))
		}
	}

	return strings.Join(r, " ")
}


func opToString( op xdr.Operation) ( opType, opContent string) {


	if op.SourceAccount != nil {
		opContent = "SRC:" + rawPublicKeyToString(*op.SourceAccount) + " "
	}
		

	switch op.Body.Type {
	case xdr.OperationTypeCreateAccount:
		opType = "Create Account"
		opContent += createAccountOpToString(op.Body.CreateAccountOp)
		
	case xdr.OperationTypePayment:
		opType = "Payment"
		opContent += paymentOpToString(op.Body.PaymentOp)

	case xdr.OperationTypePathPayment:
		opType = "Path Payment"

	case xdr.OperationTypeManageOffer:
		opType = "Manage Offer"

	case xdr.OperationTypeCreatePassiveOffer:
		opType = "Create Passive Offer"

	case xdr.OperationTypeSetOptions:
		opType = "Set Options"
		opContent += setOptionsOpToString(op.Body.SetOptionsOp)

	case xdr.OperationTypeChangeTrust:
		opType = "Change Trust"

	case xdr.OperationTypeAllowTrust:
		opType = "Allow Trust"

	case xdr.OperationTypeAccountMerge:
		opType = "Account Merge"

	case xdr.OperationTypeInflation:
		opType = "Inflation"

	case xdr.OperationTypeManageData:
		opType = "Manage Data"
	}

	return
}
	

func print_transaction( txe *xdr.TransactionEnvelope, prefix string, fp io.Writer) {
	var table [][]string

	tx := txe.Tx
	
	table = appendTableLine(table, "Source Account ID", rawPublicKeyToString(tx.SourceAccount))

	var seq big.Int
	seq.SetUint64(uint64(xdr.Uint64(tx.SeqNum)))
	
	table = appendTableLine(table, "Sequence", seq.String())

	table = appendTableLine(table, "Base Fee", amount.String(xdr.Int64(tx.Fee)))

	for _, op := range tx.Operations {
		opType, opContent := opToString( op )
		table = appendTableLine(table, opType, opContent)
	}


	for _, sig := range txe.Signatures {
		table = appendTableLine(table, "Signature", hex.EncodeToString(sig.Signature))	
	}
	
	printTablePrefixFp(table, 2, ": ", prefix, fp)

}


func readTransactionBlob( fileName string) (string, error) {
	fp, err := os.Open(fileName)

	if err != nil {
		return "", err
	}

	scan := bufio.NewScanner(fp)

	
	for scan.Scan() {
		line := scan.Text()
		
		line = strings.TrimSpace(line)

		i := strings.Index(line, "#")
		if i >= 0 {
			line = line[0:i]
		}

		if line == "" {
			continue
		}

		fp.Close()
		return line, nil
	}

	err = scan.Err()

	fp.Close()

	return "", err
}

func writeTransactionBlob( blob string, txe *xdr.TransactionEnvelope, fileName string) error {
	fp, err := os.Create(fileName)

	if err != nil {
		return err
	}

	print_transaction( txe, "#", fp)

	_, err = fmt.Fprintf( fp, "%s\n", blob)

	if err != nil {
		fp.Close()
		return err
	}
	

	err = fp.Close() 

	if err != nil {
		return err
	}

	return nil
}
