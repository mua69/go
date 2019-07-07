package main


import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/mua69/stellarwallet"
	"github.com/stellar/go/address"
	"github.com/stellar/go/amount"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/price"
	hprotocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/xdr"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)


type MenuEntry struct {
	Id string
	Prompt string
	Enabled bool
}

type MenuEntryCB struct {
	Callback func ()
	Prompt string
	Enabled bool
}

type terminalState struct {
	state *terminal.State
}

func isTerminal(fd uintptr) bool {
	return terminal.IsTerminal(int(fd))
}

func makeRaw(fd uintptr) (*terminalState, error) {
	state, err := terminal.MakeRaw(int(fd))

	return &terminalState{
		state: state,
	}, err
}

func restore(fd uintptr, oldState *terminalState) error {
	return terminal.Restore(int(fd), oldState.state)
}

var getch = func(r io.Reader) (byte, error) {
	buf := make([]byte, 1)
	if n, err := r.Read(buf); n == 0 || err != nil {
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
	return buf[0], nil
}

// Reads a Stellar public or private key from the terminal
// If a private key is entered (starting with S) the character "*" is echoed,
// except for the 1st and last 2 characters.
// Public keys are echoed verbatimly.

func getAddressFromTerminal() (string) {
	r := os.Stdin
	w := os.Stdout

	var err error
	var bs = []byte("\b \b")
	var mask = []byte("*")

	if isTerminal(r.Fd()) {
		if oldState, err := makeRaw(r.Fd()); err != nil {
			panic(err)
		} else {
			defer func() {
				restore(r.Fd(), oldState)
				fmt.Fprintln(w)
			}()
		}
	}

	// read up to 56 characters, echo '*' if input starts with 'S', i.e. is a private key
	var adr []byte
	maxLength := 56
	var masked bool

	var i = 0
	for ; ; {
		var v byte
		if v, err = getch(r); err != nil {
			break
		} 
		
		// handle backspace
		if  (v == 127 || v == 8) {
			if i > 0 {
				i--
				adr = adr[:i]
				fmt.Fprint(w, string(bs))
			}
		} else if v == 13 || v == 10 {
			break
		} else if v == 3 {
			err = errors.New("interrupted")
			break
		} else if v != 0 && i < maxLength {
			if unicode.IsLower(rune(v)) {
				v = byte(unicode.ToUpper(rune(v)))
			}
				
			if i == 0 {
				if v == 'S' {
					masked = true
				} else {
					masked = false
				}
			}

			adr = append(adr, v)
			if masked && i > 0 && i < maxLength-2 {
				fmt.Fprint(w, string(mask))
			} else {
				fmt.Fprint(w, string(v))
			}
			i++
		}
	}

	if err != nil {
		panic(err)
	}

	return string(adr)
}


// read a private key
func getSeed(prompt string, allowEmpty bool) (string) {
	var src string = ""

	for done := false; !done; {
		fmt.Printf("%s (seed/private key): ", prompt)
		pass := getAddressFromTerminal()

		if allowEmpty && pass == "" {
			return ""
		}

		kp, err := keypair.Parse(pass)
		if err != nil {
			fmt.Println("Invalid seed.")
		} else {
			kpf, ok := kp.(*keypair.Full)
			if !ok {
				fmt.Println("Public key entered.")
			} else {
				src = kpf.Seed()
				done = true
			}
		}
	}

	return src
}

// reads and returns a private or public key
func getAddressOrSeed(prompt string) (string) {
	var adr string = ""

	for done := false; !done; {
		fmt.Printf("%s (public or private key): ", prompt)

		input := getAddressFromTerminal()
		input = strings.TrimRight(input, "\r\n")
		kp, err := keypair.Parse(input)

		if err != nil {
			fmt.Println("Invalid address.")
		} else {
			kpf, ok := kp.(*keypair.Full)
			if ok {
				adr = kpf.Seed()
			} else {
				adr = kp.Address()
			}
			done = true
		}
	}

	return adr
}


// read and return a public key from the terminal
func getAddress(prompt string) (string) {
	var adr string = ""

	for done := false; !done; {
		fmt.Printf("%s (public key): ", prompt)

		input := getAddressFromTerminal()
		input = strings.TrimRight(input, "\r\n")
		kp, err := keypair.Parse(input)
		if err != nil {
			fmt.Println("Invalid address.")
		} else {
			adr = kp.Address()
			done = true
		}
	}

	return adr
}

// reads an asset id and issuer from terminal
func getAsset(prompt string) (id, issuer string) {
	issuer = getAddress(prompt + " Asset Issuer")

	for {
		id = readLine(prompt + " Asset ID")
		if id == "" {
			continue
		}

		err := stellarwallet.CheckAssetId(id)

		if err == nil {
			return
		}

		fmt.Printf("Invalid Asset ID: %s\n", err.Error())
	}

	return
}

// read native XLM payment amount from the terminal
func getPayment(prompt string) (string) {
	var amount string
	in := bufio.NewReader(os.Stdin)

	for done := false; !done; {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)

		_, err = price.Parse(input)
		if err != nil {
			fmt.Println("Invalid amount.")
		} else {
			amount = input
			done = true
		}
	}

	return amount
}

func priceToRat(price interface{}) *big.Rat {
	switch price.(type) {
	case hprotocol.Price:
		p := price.(hprotocol.Price)
		return big.NewRat(int64(p.N), int64(p.D))
	case xdr.Price:
		p := price.(xdr.Price)
		return big.NewRat(int64(p.N), int64(p.D))
	}

	return nil
}
// read price from terminal
func getPrice(prompt string) *big.Rat {
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)

		p, err := price.Parse(input)
		if err != nil {
			fmt.Println("Invalid price.")
		} else {
			return priceToRat(p)
		}
	}
}

// read amount from the terminal
func getAmount(prompt string) *big.Rat {
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)

		_, err = amount.Parse(input)
		if err != nil {
			fmt.Println("Invalid amount.")
		} else {
			return amountToRat(input)
		}
	}
}

// read a memo text from the terminal
func getMemoText(prompt string) (string) {
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)
		
		if input != "" {
			if len(input) <= 28 {
				return input
			} else {
				fmt.Println("Memo text too long, max length 28 characters.")
			}
		}
	}
	
}

func getMemoHash(prompt string) (hash [32]byte) {
	
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)
		
		val, err := hex.DecodeString(input)

		if err != nil {
			fmt.Printf("Invalid hash value (expecting 64 digit hex string): %s\n", err.Error())
		} else if len(val) != 32 {
			fmt.Printf("Invalid length of hash value (expecting 64 digits/32 bytes).\n", err.Error())
		} else {
			for i := 0; i < 32; i++ {
				hash[i] = val[i]
			}
			return
		}
	}

}


// read a federation address from the terminal
func getFederationAddress(prompt string) (string) {
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimRight(input, "\r\n")
		_, _, err = address.Split(input)
		if  err == nil {
			return input
		} else {
			fmt.Println("Invalid address (use '*' as separator).")
		}
	}
	
}

func readLine(prompt string) (string) {
	in := bufio.NewReader(os.Stdin)

	fmt.Printf("%s: ", prompt)

	input, err := in.ReadString('\n')
	if err != nil {
		panic(err)
	}
	input = strings.TrimSpace(input)
	return input
}

// read a memo ID from the terminal
func getMemoID(prompt string) (uint64) {

	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)

		id, err := strconv.ParseUint(input, 10, 64)
		if err == nil {
			return id
		} else {
			fmt.Println("Memo ID invalid, must be unsigned integer.")
		}
	}
}

// reads an unit64 number from terminal
func getUint64(prompt string) (uint64) {

	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimSpace(input)

		id, err := strconv.ParseUint(input, 10, 64)
		if err == nil {
			return id
		} else {
			fmt.Println("invalid input, must be unsigned integer.")
		}
	}
}


func getOk(prompt string) (bool) {
	in := bufio.NewReader(os.Stdin)

	for ; ; {
		fmt.Printf("%s.\nOK? (y/n): ", prompt)
		input, err := in.ReadString('\n')
		input = strings.TrimRight(input, "\r\n")
		if err != nil {
			panic(err)
		}
		if input == "y" {
			return true
		} else if input == "n" {
			return false
		} else {
			fmt.Println("Please enter y or n.")
		} 
	}
}

func getInteger(prompt string) int {
	in := bufio.NewReader(os.Stdin)

	for ; ; {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')

		if err != nil {
			panic(err)
		}

		input = strings.TrimSpace(input)

		val, err := strconv.Atoi(input)

		if err == nil {
			return val
		} else {
			fmt.Println("Please enter an integer value.")	
		}		
	}
}

func getPasswordFromTerminal(p *string) {
	r := os.Stdin
	w := os.Stdout

	var err error
	var bs = []byte("\b \b")
	var mask = []byte("*")

	if isTerminal(r.Fd()) {
		if oldState, err := makeRaw(r.Fd()); err != nil {
			panic(err)
		} else {
			defer func() {
				restore(r.Fd(), oldState)
				fmt.Fprintln(w)
			}()
		}
	}

	var pw []byte

	var i = 0
	for ; ; {
		var v byte
		if v, err = getch(r); err != nil {
			break
		} 
		
		// handle backspace
		if  (v == 127 || v == 8) {
			if i > 0 {
				i--
				pw = pw[:i]
				fmt.Fprint(w, string(bs))
			}
		} else if v == 13 || v == 10 {
			break
		} else if v == 3 {
			err = errors.New("interrupted")
			break
		} else if v != 0 {
			pw = append(pw, v)
			fmt.Fprint(w, string(mask))
			i++
		}
	}

	if err != nil {
		panic(err)
	}

	*p = string(pw)

}


func getPassword(prompt string, nonEmpty bool, pw *string) {
	for {
		fmt.Printf("%s: ", prompt)
		getPasswordFromTerminal(pw)
		if !nonEmpty || *pw != "" {
			break
		}
		fmt.Print("Empty password entered.\n")
	}
}



func getPasswordWithConfirmation(prompt string, nonEmpty bool, pw *string) {
	var pw1, pw2 string

	for {
		getPassword(prompt, nonEmpty, &pw1)
		getPassword("Confirm " + prompt, nonEmpty, &pw2)
		if pw1 != pw2 {
			fmt.Print("Passwords do not match.\n")
		} else {
			break
		}
	}

	*pw = pw1
}

func appendTableLine( table [][]string, str ...string) [][]string {
	var line []string

	for _, s := range str {
		line = append(line, s)
	}

	table = append(table, line)
	return table
}

func printTable(table [][]string, cols int, separator string) {
	printTablePrefixFp(table, cols, separator, "", os.Stdout)
}

func printTablePrefixFp( table [][]string, cols int, separator string, prefix string, fp io.Writer) {
	var colw = make([]int, cols, cols)

	for i := range colw {
		colw[i] = 0
	}

	for l := range table {
		line := table[l]
		for c := range line {
			if c < cols {
				len := utf8.RuneCountInString(line[c])
				if len > colw[c] {
					colw[c] = len
				}
			}
		}
	}
			
	for l := range table {
		line := table[l]
		fmt.Fprintf(fp, "%s", prefix)
		for c := range line {
			len := utf8.RuneCountInString(line[c])
			fmt.Fprintf(fp, "%s%s", line[c], strings.Repeat(" ", colw[c]-len))
			if c < cols-1 {
				fmt.Fprintf(fp, "%s", separator)
			}
		}
		fmt.Fprintf(fp, "\n")
	}
}


func runMenu(menu []MenuEntry, quitChoice bool) string {

	var table [][]string

	choices := make([]string, len(menu))
	
	choice := 1
	for i := 0; i < len(menu); i++ {
		if menu[i].Enabled {
			choices[i] = fmt.Sprintf("%d", choice)
			table = appendTableLine( table, choices[i], menu[i].Prompt )
			choice++
		}
	}
	
	if quitChoice {
		table = appendTableLine( table, "q", "Quit" )
	}

	printTable(table, 2, " ")

	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n--> ")
		input, err := in.ReadString('\n')
		
		if err != nil {
			panic(err)
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if quitChoice && input == "q" {
			fmt.Println("Quit.")
			os.Exit(0)
		}

		for i, _ := range choices {
			if input == choices[i] {
				return menu[i].Id
			}
		}

		fmt.Printf("Invalid input: %s\n", input)
	}
	

}

func runCallbackMenu(menu []MenuEntryCB, prompt string, loop bool) {

	var table [][]string

	choices := make([]string, len(menu))
	
	choice := 1
	for i := 0; i < len(menu); i++ {
		if menu[i].Enabled {
			choices[i] = fmt.Sprintf("%d", choice)
			table = appendTableLine( table, choices[i], menu[i].Prompt )
			choice++
		}
	}
	
	table = appendTableLine( table, "q", "Done" )

	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("\n%s:\n", prompt)
		printTable(table, 2, " ")

		for {
			fmt.Printf("\n--> ")
			input, err := in.ReadString('\n')
		
			if err != nil {
				panic(err)
			}

			input = strings.TrimSpace(input)

			if input == "" {
				continue
			}

			if input == "q" {
				return
			}

			found := false
			for i, _ := range choices {
				if input == choices[i] {
					found = true
					menu[i].Callback()
					if !loop {
						return
					}
				}
			}

			if found {
				break
			} else {
				fmt.Printf("Invalid input: %s\n", input)
			}
		}
	}
}

type CliTable struct {
	cols int
	lines [][]string
	justification []int
	separator []string
	columnWidths []int
	prefix string
	fp io.Writer
}

const CliTableJustificationLeft = 0
const CliTableJustificationRight = 1
const CliTableJustificationCenter = 2

func newCliTable(cols int) *CliTable {
	t := new(CliTable)
	if cols < 1 {
		panic(fmt.Sprintf("invalid number of columns: %s", cols))
	}
	t.cols = cols
	t.fp = os.Stdout
	t.justification = make([]int, cols)
	t.separator = make([]string, cols)
	t.lines = make([][]string, 0, 20)
	t.columnWidths = make([]int, cols)

	for i := 0; i < cols; i++ {
		t.justification[i] = CliTableJustificationLeft
		t.separator[i] = " "
	}

	return t
}

func (t *CliTable)appendLine(s... string) {
	t.lines = append(t.lines, s)

	for i := range s {
		if i < t.cols {
			l := utf8.RuneCountInString(s[i])
			if l > t.columnWidths[i] {
				t.columnWidths[i] = l
			}
		}
	}
}

func (t *CliTable)setJustification(justification... int) {
	for i := range justification {
		if i < t.cols {
			t.justification[i] = justification[i]
		}
	}
}

func (t *CliTable)setSeparator(sep... string) {
	for i := range sep {
		if i < t.cols {
			t.separator[i] = sep[i]
		}
	}
}

func (t *CliTable)print() {
	for l := range t.lines {
		line := t.lines[l]
		fmt.Fprintf(t.fp, "%s", t.prefix)
		for c := range line {
			if c < t.cols {
				len := utf8.RuneCountInString(line[c])
				n := t.columnWidths[c] - len
				switch t.justification[c] {
				case CliTableJustificationRight:
					fmt.Fprintf(t.fp, "%s%s", strings.Repeat(" ", n), line[c])
				case CliTableJustificationCenter:
					n1 := n/2
					fmt.Fprintf(t.fp, "%s%s", strings.Repeat(" ", n1), line[c], strings.Repeat(" ", n-n1))
				default:
					fmt.Fprintf(t.fp, "%s%s", line[c], strings.Repeat(" ",  n))
				}
				fmt.Fprintf(t.fp, "%s", t.separator[c])
			}
		}
		fmt.Fprintf(t.fp, "\n")
	}
}
