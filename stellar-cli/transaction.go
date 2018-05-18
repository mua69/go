package main

import (
	"errors"
	"os"
	"fmt"
	"io"
	"bufio"
	"strings"
	"strconv"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"github.com/stellar/go/build"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
)


func loadAccount(adr string) (*horizon.Account, error) {

	kp := keypair.MustParse(adr)

	acc, err := g_horizon.LoadAccount(kp.Address())

	if err != nil {
		if herr, ok := err.(*horizon.Error); ok {
			if herr.Problem.Title == "Resource Missing" {
				return nil, nil
			}
		}

		return nil, err
	}

	return &acc, nil
}

/*
type TxObject struct {
	Embedded struct {
		Records []horizon.Transaction
	} `json:"_embedded"`
}
*/

func getAccountTransactions(adr string, cnt int, pagingTokenIn string) (txs []horizon.Transaction, pagingTokenOut string, err error) {
	if cnt <= 0 {
		return
	}

	acc, err := loadAccount(adr)

	if acc == nil {
		return
	}

	var obj struct {
		Embedded struct {
			Records []horizon.Transaction
		} `json:"_embedded"`
	}


	baseUrl := strings.TrimRight(g_horizon.URL, "/")
	baseUrl += "/accounts/" + adr + "/transactions"

	if cnt > 200 {
		cnt = 200 // maximum limit allowed by horizon server
	}

	var url string

	if pagingTokenIn != "" {
		url = fmt.Sprintf("%s?order=desc&limit=%d&cursor=%s", baseUrl, cnt, pagingTokenIn)
	} else {
		url = fmt.Sprintf("%s?order=desc&limit=%d", baseUrl, cnt)
	}


	//fmt.Println(url)
	
	resp, err := g_horizon.HTTP.Get(url)
	if err != nil {
		return
	}
		
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
		
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		horizonError := &horizon.Error{
			Response: resp,
		}
		decodeError := decoder.Decode(&horizonError.Problem)
		if decodeError != nil {
			err = errors.New("error decoding horizon.Problem")
			return
		}
		err = horizonError
		return
	}

	err = decoder.Decode(&obj)
	if err != nil {
		return
	}

	n := len(obj.Embedded.Records) 
	
	if n == 0 {
		return
	}
	
	if n >= cnt {
		// there are probably more transaction records, return paging token of last transaction
		pagingTokenOut = obj.Embedded.Records[n-1].PagingToken
	}

	txs = obj.Embedded.Records

	return
}

func printHorizonError(action string, err error) {
	if err != nil {
		fmt.Println("%s failed, error details:", action)
		if herr, ok := err.(*horizon.Error); ok {
			fmt.Println(herr.Problem.Title)
			fmt.Println(herr.Problem.Detail)
			fmt.Println(string(herr.Problem.Extras["result_codes"]))
			fmt.Println(herr.Error())
			
		} else {
			fmt.Println(err.Error())
		}
	}
}


func printTransactionResults( txr horizon.TransactionSuccess) {
	fmt.Println("Transaction posted in ledger: ", txr.Ledger)
	fmt.Println("Transaction hash            : ", txr.Hash)
}


func tx_setup( src string ) (tx *build.TransactionBuilder) {
	acc, err := loadAccount(src)

	if err != nil {
		panic(err)
	}

	if acc == nil {
		// account does not exist
		return nil
	}

	seq, err := strconv.ParseUint(acc.Sequence, 10, 64)

	if err != nil {
		fmt.Println("Failed to parse account sequence number.")
		panic(err)
	}

	tx, err = build.Transaction(
		build.SourceAccount{src},
		build.Sequence{seq+1},
		g_network)

	if err != nil {
		panic(err)
	}

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

func tx_inflationDestination( tx *build.TransactionBuilder, dst string) {
	tx.Mutate(build.SetOptions(build.InflationDest(dst)))
}

func tx_addTrustLine( tx *build.TransactionBuilder, asset horizon.Asset) {
	tx.Mutate(build.Trust(asset.Code, asset.Issuer))
}

func horizonAssetToBuildAsset(a horizon.Asset) build.Asset {
	if a.Type == "native" {
		return build.NativeAsset()
	} else {
		return build.CreditAsset(a.Code, a.Issuer)
	}
}

func tx_addOrder(tx *build.TransactionBuilder, selling, buying horizon.Asset, price, amount string) {
	tx.Mutate(build.CreateOffer(build.Rate{horizonAssetToBuildAsset(selling),
		horizonAssetToBuildAsset(buying), build.Price(price)},
		build.Amount(amount)))
}

func tx_updateOrder(tx *build.TransactionBuilder, selling, buying horizon.Asset, price, amount string, orderid uint64) {
	tx.Mutate(build.UpdateOffer(build.Rate{horizonAssetToBuildAsset(selling),
		horizonAssetToBuildAsset(buying), build.Price(price)},
		build.Amount(amount), build.OfferID(orderid)))
}

func tx_memoText( tx *build.TransactionBuilder, memoText string ) {
	tx.Mutate(build.MemoText{memoText})
}

func tx_memoID( tx *build.TransactionBuilder, memoID uint64 ) {
	tx.Mutate(build.MemoID{memoID})
}

func tx_memoHash( tx *build.TransactionBuilder, memoHash [32]byte ) {
	tx.Mutate(build.MemoHash{memoHash})
}

func tx_memoRetHash( tx *build.TransactionBuilder, memoHash [32]byte ) {
	tx.Mutate(build.MemoReturn{memoHash})
}

func tx_sign( tx *build.TransactionBuilder) (bool, build.TransactionEnvelopeBuilder) {
	txe, err :=  tx.Sign()
	if err != nil {
		panic(err)
	}

	if len(g_signers) > 0 {
		for _, s := range g_signers {
			txe.Mutate(build.Sign{s})
		}
		return true, txe
	} else {
		return false, txe
	}
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
	resp, err := g_horizon.SubmitTransaction(tx_blob)
	if err != nil {
		fmt.Println("Failed to submit transaction. Horizon error details:")
		if herr, ok := err.(*horizon.Error); ok {
			fmt.Println(herr.Problem.Title)
			fmt.Println(herr.Problem.Detail)
			fmt.Println(string(herr.Problem.Extras["result_codes"]))
			fmt.Println(herr.Error())

		} else {
			fmt.Println(err.Error())
		}
	} else {
		printTransactionResults(resp)
	}
}


func rawPublicKeyToString( k xdr.AccountId) string {
	var b32 [32]byte = *k.Ed25519

	return strkey.MustEncode(strkey.VersionByteAccountID, b32[:])
}

func xdrAssetToString(a xdr.Asset) string {
	switch a.Type {
	case xdr.AssetTypeAssetTypeCreditAlphanum4:
		return string(a.AlphaNum4.AssetCode[:]) + "/" + rawPublicKeyToString(a.AlphaNum4.Issuer)

	case xdr.AssetTypeAssetTypeCreditAlphanum12:
		return string(a.AlphaNum12.AssetCode[:]) + "/" + rawPublicKeyToString(a.AlphaNum12.Issuer)
	}

	return "XLM"

}

func changeTrustOpToString(op *xdr.ChangeTrustOp) string {
	s := "ASSET:" + xdrAssetToString(op.Line)

	if op.Limit != math.MaxInt64 {
		s += " LIMIT:" + amount.StringFromInt64(int64(op.Limit))
	}

	return s
}

func manageOfferOpToString(op *xdr.ManageOfferOp) string {
	return "SELL:" + xdrAssetToString(op.Selling) + " BUY:" + xdrAssetToString(op.Buying) +
		" AMOUNT:" + amount.StringFromInt64(int64(op.Amount)) +
		" PRICE:" + op.Price.String() + " ID:" + fmt.Sprintf("%d", op.OfferId)
}

func paymentOpToString( op *xdr.PaymentOp) string {
	return "DST:" + rawPublicKeyToString(op.Destination) + " AMT:" + op.Asset.String() + ":" + amount.String(op.Amount)
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

func allowTrustOpToString(op *xdr.AllowTrustOp) string {
	var s = "TRUSTOR:" + rawPublicKeyToString(op.Trustor)

	s += " ASSET:"

	switch op.Asset.Type {
	case xdr.AssetTypeAssetTypeCreditAlphanum4:
		s += string(op.Asset.AssetCode4[:])

	case xdr.AssetTypeAssetTypeCreditAlphanum12:
		s += string(op.Asset.AssetCode12[:])

	default:
		s += "XLM"
	}

	s += " AUTH:"

	if op.Authorize {
		s += "TRUE"
	} else {
		s += "FALSE"
	}

	return s
}

func opToString( op xdr.Operation ) ( opType, opContent string) {


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
		opContent += manageOfferOpToString(op.Body.ManageOfferOp)

	case xdr.OperationTypeCreatePassiveOffer:
		opType = "Create Passive Offer"

	case xdr.OperationTypeSetOptions:
		opType = "Set Options"
		opContent += setOptionsOpToString(op.Body.SetOptionsOp)

	case xdr.OperationTypeChangeTrust:
		opType = "Change Trust"
		opContent += changeTrustOpToString(op.Body.ChangeTrustOp)

	case xdr.OperationTypeAllowTrust:
		opType = "Allow Trust"
		opContent += allowTrustOpToString(op.Body.AllowTrustOp)

	case xdr.OperationTypeAccountMerge:
		opType = "Account Merge"

	case xdr.OperationTypeInflation:
		opType = "Inflation"

	case xdr.OperationTypeManageData:
		opType = "Manage Data"

	default:
		opType = "Unknown operation type"
	}

	return
}

func paymentToStringPretty( op xdr.Operation, src, acc string)  ( opType, opContent string) {

	if op.SourceAccount != nil {
		src = rawPublicKeyToString(*op.SourceAccount)
	}

	pop := op.Body.PaymentOp
	opType = "Payment"

	dst := rawPublicKeyToString(pop.Destination) 

	if acc == src {
		opContent = "TO   " + dst
	} else if acc == dst {
		opContent = "FROM " + src
	} else {
		opType = ""
		return
	}

	asset :=  pop.Asset.String()
	if asset == "native" {
		asset = "XLM"
	}

	opContent += " " + asset + " " + amount.String(pop.Amount)

	return
}

func memoToString( memo xdr.Memo) (mtype, mstr string) {
	switch memo.Type {
	case xdr.MemoTypeMemoNone:
		mtype = "NONE"
		mstr = ""

	case xdr.MemoTypeMemoText:
		mtype = "TEXT"
		mstr = *memo.Text

	case xdr.MemoTypeMemoId:
		mtype = "ID"
		mstr = strconv.FormatUint(uint64(*memo.Id), 10)

	case xdr.MemoTypeMemoHash:
		mtype = "HASH"
		mstr = hex.EncodeToString((*memo.Hash)[:])

	case xdr.MemoTypeMemoReturn:
		mtype = "RETURN_HASH"
		mstr = hex.EncodeToString((*memo.RetHash)[:])
	}

	return
}

func print_transaction( txe *xdr.TransactionEnvelope, prefix string, fp io.Writer) {
	var table [][]string

	tx := txe.Tx
	
	table = appendTableLine(table, "Source Account", rawPublicKeyToString(tx.SourceAccount))

	for _, op := range tx.Operations {
		opType, opContent := opToString( op )
		table = appendTableLine(table, opType, opContent)
	}

	mtype, mstr := memoToString(tx.Memo)

	if mstr != "" {
		table = appendTableLine(table, "Memo", mtype + ":" + mstr)
	} else {
		table = appendTableLine(table, "Memo", mtype)
	}

	table = appendTableLine(table, "Base Fee", amount.String(xdr.Int64(tx.Fee)))

	var seq big.Int
	seq.SetUint64(uint64(xdr.Uint64(tx.SeqNum)))
	table = appendTableLine(table, "Sequence", seq.String())


	for _, sig := range txe.Signatures {
		table = appendTableLine(table, "Signature", hex.EncodeToString(sig.Signature))	
	}
	
	printTablePrefixFp(table, 2, ": ", prefix, fp)

}

func pretty_print_transaction( txe *xdr.TransactionEnvelope, acc string) {
	var table [][]string

	tx := txe.Tx
	
	txSrcAcc := rawPublicKeyToString(tx.SourceAccount)

	for _, op := range tx.Operations {

		var opType, opContent string

		if op.Body.Type == xdr.OperationTypePayment {
			opType, opContent = paymentToStringPretty(op, txSrcAcc, acc)
		} else { 
			opType, opContent = opToString( op )
		}
		
		if opType != "" {
			table = appendTableLine(table, opType, opContent)
		}
	}

	mtype, mstr := memoToString(tx.Memo)

	if mtype != "NONE" {
		table = appendTableLine(table, "Memo", mstr)
	}
	
	printTable(table, 2, ": ")
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

// read signers (private keys) from given file.
// '#' used for comments
// lines not containing a valid private key are silently ignored
func readSignersFile( fileName string) (cnt int, err error) {
	cnt = 0
	fp, err := os.Open(fileName)

	if err != nil {
		return cnt, err
	}

	scan := bufio.NewScanner(fp)

	
	for scan.Scan() {
		line := scan.Text()
		err = scan.Err()

		if err != nil {
			fp.Close()
			return cnt, err
		}


		i := strings.Index(line, "#")
		if i >= 0 {
			line = line[0:i]
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		kp, err := keypair.Parse(line)
		if err == nil {
			kpf, ok := kp.(*keypair.Full)

			if ok {
				if len(g_signers) < 20 {

					g_signers = append(g_signers, kpf.Seed())
					cnt++
				}
			}
		}
	}

	fp.Close()

	return cnt, nil
}

func readSignersFromFile() int {
	if g_signersFile == "" {
		return 0
	}

	cnt, err := readSignersFile(g_signersFile)

	if err == nil {
		fmt.Printf("Read %d signing key(s) from file \"%s\".\n", cnt, g_signersFile)
	} else {
		fmt.Printf("Failed to read signers file: %s\n", err.Error())
	}

	return cnt
}

func readSigners() {
	for cnt:= 0;  len(g_signers) < 20 ; cnt++ {
		var seed string

		seed = getSeed("Additional private signing key (hit enter to skip)", true)			

		if seed == "" {
			return
		}

		g_signers = append(g_signers, seed)
	}
}
