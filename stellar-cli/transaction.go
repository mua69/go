package main

import (
	"bufio"
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


func printTransactionResults( txr hprotocol.TransactionSuccess) {
	fmt.Println("Transaction posted in ledger: ", txr.Ledger)
	fmt.Println("Transaction hash            : ", txr.Hash)
}

type Transaction struct {
	tx *txnbuild.Transaction
	signed bool
}

func newTransaction(src string) *Transaction {
	acc := getAccountInfo(src, CacheTimeoutForce)


	if !acc.exists {
		// account does not exist
		return nil
	}

	t := new(Transaction)
	t.tx = new(txnbuild.Transaction)

	t.tx.SourceAccount = acc.horizonData

	t.tx.Network = g_network

	t.signed = false

	return t
}

func (t *Transaction)isSigned() bool {
	return t.signed
}

func (t *Transaction) sign() bool {

	t.tx.Timebounds = txnbuild.NewTimeout(60)

	err := t.tx.Build()

	if err != nil {
		fmt.Printf("Failed to build transaction: %v", err)
		return false
	}

	signers := make([]*keypair.Full, 0)

	for _, s := range g_signers {
		kp := keypair.MustParse(s)
		kpf := kp.(*keypair.Full)

		signers = append(signers, kpf)
	}

	err = t.tx.Sign(signers...)

	if err != nil {
		fmt.Printf("Failed to sign transaction: %v", err)
		return false
	}

	if len(g_signers) > 0 {
		t.signed = true
	}

	return true
}

func (t *Transaction)getBlob() string {
	s, err := t.tx.Base64()

	if err != nil {
		fmt.Printf("Failed to serialize transaction: %v", err)
		return ""
	}

	return s
}

func (t *Transaction) getXdrEnvelope() *xdr.TransactionEnvelope {
	txe_xdr := &xdr.TransactionEnvelope{ }

	txBlob := t.getBlob()

	if txBlob == "" {
		return nil
	}

	err := txe_xdr.Scan(txBlob)

	if err != nil {
		fmt.Printf("Failed to scan transaction blob: %v", err)
		return nil
	}

	if txe_xdr.Tx.SourceAccount.Ed25519 == nil {
		return nil
	}

	return txe_xdr
}

func (t *Transaction) getHash() (error, [32]byte) {
	h, err := t.tx.Hash()

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
	op := txnbuild.CreateAccount{Destination:dst, Amount:amount}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) nativePayment(dst string, amount string) {
	op:= txnbuild.Payment{Destination:dst, Amount:amount, Asset:txnbuild.NativeAsset{}}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) assetPayment(dst string, asset *Asset, amount *big.Rat) {
	op := txnbuild.Payment{Destination:dst, Amount:amountToString(amount),
		Asset:txnbuild.CreditAsset{asset.Code(), asset.Issuer()}}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) inflationDestination(dst string) {
	op := txnbuild.SetOptions{InflationDestination:txnbuild.NewInflationDestination(dst)}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) addTrustLine( asset *Asset) {
	op := txnbuild.ChangeTrust{Line:txnbuild.CreditAsset{asset.Code(), asset.Issuer()}, Limit:txnbuild.MaxTrustlineLimit}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) removeTrustLine( asset *Asset) {
	op := txnbuild.RemoveTrustlineOp(txnbuild.CreditAsset{asset.Code(), asset.Issuer()})

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) addSellOrder(selling, buying *Asset, price, amount *big.Rat, orderid uint64) {
	op := txnbuild.ManageSellOffer{
		Selling:selling.toCreditAsset(),
		Buying:buying.toCreditAsset(),
		Amount:amountToString(amount),
		Price:price.FloatString(10),
		OfferID:int64(orderid)	}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) addBuyOrder(selling, buying *Asset, price, amount *big.Rat, orderid uint64) {
	op := txnbuild.ManageBuyOffer{
		Selling:selling.toCreditAsset(),
		Buying:buying.toCreditAsset(),
		Amount:amountToString(amount),
		Price:price.FloatString(10),
		OfferID:int64(orderid)	}

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) cancelOrder(orderid uint64) {
	op, _ := txnbuild.DeleteOfferOp(int64(orderid))

	t.tx.Operations = append(t.tx.Operations, &op)
}

func (t *Transaction) memoText( memoText string ) {
	t.tx.Memo = txnbuild.MemoText(memoText)
}

func (t *Transaction) memoID( memoID uint64 ) {
	t.tx.Memo = txnbuild.MemoID(memoID)
}

func (t *Transaction) memoHash( memoHash [32]byte ) {
	t.tx.Memo = txnbuild.MemoHash(memoHash)
}

func (t *Transaction) memoRetHash( memoHash [32]byte ) {
	t.tx.Memo = txnbuild.MemoReturn(memoHash)
}






func tx_transmit_blob( tx_blob string ) {
	resp, err := g_horizon.SubmitTransactionXDR(tx_blob)
	if err != nil {
		fmt.Println("Failed to submit transaction. Horizon error details:")
		printHorizonError("submit transaction", err)
	} else {
		printTransactionResults(resp)
	}
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

func writeTransactionBlob( blob string, tx *Transaction, fileName string) error {
	fp, err := os.Create(fileName)

	if err != nil {
		return err
	}

	txe := tx.getXdrEnvelope()

	if txe == nil {
		return errors.New("Failed to decode transaction.")
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
