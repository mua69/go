package main

import (
	"encoding/hex"
	"fmt"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
)

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

func manageSellOfferOpToString(op *xdr.ManageSellOfferOp) string {
	return "SELL:" + xdrAssetToString(op.Selling) + " BUY:" + xdrAssetToString(op.Buying) +
		" AMOUNT:" + amount.StringFromInt64(int64(op.Amount)) +
		" PRICE:" + op.Price.String() + " ID:" + fmt.Sprintf("%d", op.OfferId)
}

func manageBuyOfferOpToString(op *xdr.ManageBuyOfferOp) string {
	return "BUY:" + xdrAssetToString(op.Buying) + " SELL:" + xdrAssetToString(op.Selling) +
		" AMOUNT:" + amount.StringFromInt64(int64(op.BuyAmount)) +
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

	case xdr.OperationTypeManageSellOffer:
		opType = "Manage Sell Offer"
		opContent += manageSellOfferOpToString(op.Body.ManageSellOfferOp)

	case xdr.OperationTypeManageBuyOffer:
		opType = "Manage Buy Offer"
		opContent += manageBuyOfferOpToString(op.Body.ManageBuyOfferOp)

	case xdr.OperationTypeCreatePassiveSellOffer:
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

