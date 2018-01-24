package main

import (
	"errors"
	"unicode"
	"strconv"
	"fmt"
	"strings"
	"io"
	"os"
	"bufio"
	"unicode/utf8"
	"golang.org/x/crypto/ssh/terminal"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/price"
	"github.com/stellar/go/address"
)



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


func getSeed(prompt string) (string) {
	var src string = ""

	for done := false; !done; {
		fmt.Printf("%s (seed/private key): ", prompt)
		pass := getAddressFromTerminal()
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

func getPayment(prompt string) (string) {
	var amount string
	in := bufio.NewReader(os.Stdin)

	for done := false; !done; {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimRight(input, "\r\n")

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

func getMemoText(prompt string) (string) {
	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimRight(input, "\r\n")
		if len(input) <= 28 {
			return input
		} else {
			fmt.Println("Memo text too long, max length 28 characters.")
		}
	}
	
}

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
	input = strings.TrimRight(input, "\r\n")
	return input
}

func getMemoID(prompt string) (uint64) {

	in := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s: ", prompt)

		input, err := in.ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimRight(input, "\r\n")
		if len(input) == 0 {
			return 0
		}
		id, err := strconv.ParseUint(input, 10, 64)
		if err == nil {
			return id
		} else {
			fmt.Println("Memo ID invalid, must be unsigned integer.")
		}
	}
}
	

func getOk(prompt string) (bool) {
	in := bufio.NewReader(os.Stdin)

	for ; ; {
		fmt.Printf("%s. OK? (y/n): ", prompt)
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
