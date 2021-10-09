package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	hprotocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"math/big"
	"os"
	"strings"
)

func printHorizonError(action string, err error) {
	if err != nil {
		fmt.Printf("%s failed, error details:\n", action)
		if herr, ok := err.(*horizonclient.Error); ok {
			resCodes, err := herr.ResultCodes()
			if err == nil {
				fmt.Printf("%s: %s %s\n", herr.Problem.Title,
					resCodes.TransactionCode, strings.Join(resCodes.OperationCodes, " "))
			} else {
				fmt.Printf("%s\n", herr.Problem.Title)
			}
		} else {
			fmt.Println(err.Error())
		}
	}
}

func printTransactionResults(tx hprotocol.Transaction) {
	fmt.Println("Transaction posted in ledger: ", tx.Ledger)
	fmt.Println("Transaction hash            : ", tx.Hash)
}

func decodeXdrTransaction(s string) (*xdr.TransactionEnvelope, error) {
	reader := strings.NewReader(s)
	b64r := base64.NewDecoder(base64.StdEncoding, reader)

	var txe xdr.TransactionEnvelope
	_, err := xdr.Unmarshal(b64r, &txe)

	return &txe, err
}

type Transaction struct {
	tx            *txnbuild.Transaction
	sourceAccount txnbuild.Account
	memo		  txnbuild.Memo
	ops           []txnbuild.Operation
	signed        bool
}

func newTransaction(src string) *Transaction {
	acc := getAccountInfo(src, CacheTimeoutForce)

	if !acc.exists {
		// account does not exist
		return nil
	}

	t := new(Transaction)
	t.sourceAccount = acc.horizonData
	t.tx = nil
	t.signed = false

	return t
}

func (t *Transaction) isSigned() bool {
	return t.signed
}

func (t *Transaction) sign() bool {

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        t.sourceAccount,
			IncrementSequenceNum: true,
			BaseFee:              txnbuild.MinBaseFee,
			Timebounds:           txnbuild.NewTimeout(300),
			Operations:           t.ops,
		Memo: t.memo})

	if err != nil {
		fmt.Printf("Failed to create transaction: %v\n", err)
		return false
	}

	signers := make([]*keypair.Full, 0)

	for _, s := range g_signers {
		kp := keypair.MustParse(s)
		kpf := kp.(*keypair.Full)

		signers = append(signers, kpf)
	}

	t.tx, err = tx.Sign(g_network, signers...)

	if err != nil {
		fmt.Printf("Failed to sign transaction: %v", err)
		return false
	}

	if len(g_signers) > 0 {
		t.signed = true
	}

	return true
}

func (t *Transaction) getBlob() string {
	if t.tx == nil {
		return ""
	}

	s, err := t.tx.Base64()

	if err != nil {
		fmt.Printf("Failed to serialize transaction: %v", err)
		return ""
	}

	return s
}

func (t *Transaction) getXdrEnvelope() *xdr.TransactionEnvelope {
	txBlob := t.getBlob()

	if txBlob == "" {
		return nil
	}

	rawr := strings.NewReader(txBlob)
	b64r := base64.NewDecoder(base64.StdEncoding, rawr)

	var txe xdr.TransactionEnvelope
	_, err := xdr.Unmarshal(b64r, &txe)


	if err != nil {
		fmt.Printf("Failed to scan transaction blob: %v", err)
		return nil
	}

	return &txe
}

func (t *Transaction) getHash() (error, [32]byte) {
	if t.tx == nil {
		return errors.New("tx is nil"), [32]byte{}
	}
	h, err := t.tx.Hash(g_network)

	if err != nil {
		fmt.Printf("Failed to hash transaction: %v", err)
		return err, h
	}

	return nil, h
}

func (t *Transaction) transmit() error {
	txeB64, err := t.tx.Base64()

	if err != nil {
		fmt.Printf("Failed to encode transaction: %v", err)
		return err
	}

	tx_transmit_blob(txeB64)

	return nil
}

func (t *Transaction) createAccount(dst string, amount string) {
	op := txnbuild.CreateAccount{Destination: dst, Amount: amount}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) nativePayment(dst string, amount string) {
	op := txnbuild.Payment{Destination: dst, Amount: amount, Asset: txnbuild.NativeAsset{}}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) assetPayment(dst string, asset *Asset, amount *big.Rat) {
	op := txnbuild.Payment{Destination: dst, Amount: amountToString(amount),
		Asset: txnbuild.CreditAsset{asset.Code(), asset.Issuer()}}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) inflationDestination(dst string) {
	op := txnbuild.SetOptions{InflationDestination: txnbuild.NewInflationDestination(dst)}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) addTrustLine(asset *Asset) {
	op := txnbuild.ChangeTrust{Line: txnbuild.CreditAsset{asset.Code(), asset.Issuer()}, Limit: txnbuild.MaxTrustlineLimit}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) removeTrustLine(asset *Asset) {
	op := txnbuild.RemoveTrustlineOp(txnbuild.CreditAsset{asset.Code(), asset.Issuer()})

	t.ops = append(t.ops, &op)
}

func (t *Transaction) addSellOrder(selling, buying *Asset, price, amount *big.Rat, orderid uint64) {
	op := txnbuild.ManageSellOffer{
		Selling: selling.toCreditAsset(),
		Buying:  buying.toCreditAsset(),
		Amount:  amountToString(amount),
		Price:   price.FloatString(10),
		OfferID: int64(orderid)}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) addBuyOrder(selling, buying *Asset, price, amount *big.Rat, orderid uint64) {
	op := txnbuild.ManageBuyOffer{
		Selling: selling.toCreditAsset(),
		Buying:  buying.toCreditAsset(),
		Amount:  amountToString(amount),
		Price:   price.FloatString(10),
		OfferID: int64(orderid)}

	t.ops = append(t.ops, &op)
}

func (t *Transaction) cancelOrder(orderid uint64) {
	op, _ := txnbuild.DeleteOfferOp(int64(orderid))

	t.ops = append(t.ops, &op)
}

func (t *Transaction) claimClaimableBalacne(balanceId string) {
	op :=txnbuild.ClaimClaimableBalance{BalanceID: balanceId}
	t.ops = append(t.ops, &op)
}

func (t *Transaction) memoText(memoText string) {
	t.memo = txnbuild.MemoText(memoText)
}

func (t *Transaction) memoID(memoID uint64) {
	t.memo = txnbuild.MemoID(memoID)
}

func (t *Transaction) memoHash(memoHash [32]byte) {
	t.memo = txnbuild.MemoHash(memoHash)
}

func (t *Transaction) memoRetHash(memoHash [32]byte) {
	t.memo = txnbuild.MemoReturn(memoHash)
}

func tx_transmit_blob(tx_blob string) {
	resp, err := g_horizon.SubmitTransactionXDR(tx_blob)
	if err != nil {
		fmt.Println("Failed to submit transaction. Horizon error details:")
		printHorizonError("submit transaction", err)
	} else {
		printTransactionResults(resp)
	}
}

func readTransactionBlob(fileName string) (string, error) {
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

func writeTransactionBlob(blob string, tx *Transaction, fileName string) error {
	fp, err := os.Create(fileName)

	if err != nil {
		return err
	}

	txe := tx.getXdrEnvelope()

	if txe == nil {
		return errors.New("Failed to decode transaction.")
	}

	print_transaction(txe, "#", fp)

	_, err = fmt.Fprintf(fp, "%s\n", blob)

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
func readSignersFile(fileName string) (cnt int, err error) {
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
	for cnt := 0; len(g_signers) < 20; cnt++ {
		var seed string

		seed = getSeed("Additional private signing key (hit enter to skip)", true)

		if seed == "" {
			return
		}

		g_signers = append(g_signers, seed)
	}
}
