package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// sep defines the separator line for ledger file
// anything below this line will be replaced by the output of `hledger print`
const sep = "; :::Transactions:::"

var (
	reTransaction = regexp.MustCompile(`^\d`)
	rePosting     = regexp.MustCompile(`^\s+\S`)
	reInclude     = regexp.MustCompile(`^include `)
)

// parseJournal splits the journal read from r into preamble and transactions
// return an error if
// - ledgerFile has no separator line
// - ledgerFile has more than one separator line
// - lines below the separator line are anything else than transactions
func parseJournal(r io.Reader) (preamble []string, transactions []string, err error) {
	// Read lines up to sep
	scan := bufio.NewScanner(r)
	foundSep := false
	lineNr := 0
	for scan.Scan() {
		lineNr++
		if scan.Text() == sep {
			foundSep = true
			break
		}
		preamble = append(preamble, scan.Text())
	}

	if !foundSep {
		return nil, nil, errors.New("ledger file contains no transaction separator")
	}

	// Check that anything after this is only valid transactions
	for scan.Scan() {
		lineNr++
		if scan.Text() == sep {
			return nil, nil, errors.New("ledger file contains multiple transaction separators")
		}

		transactions = append(transactions, scan.Text())

		// Allow empty lines
		if strings.TrimSpace(scan.Text()) == "" {
			continue
		}

		// Date - starts transaction
		if reTransaction.Match(scan.Bytes()) {
			continue
		}

		// posting lines - must be indented
		if rePosting.Match(scan.Bytes()) {
			continue
		}

		return nil, nil, fmt.Errorf("ledger file contains unexpected line %d in transactions:\n%s", lineNr, scan.Text())
	}

	if err := scan.Err(); err != nil {
		return nil, nil, scan.Err()
	}
	return preamble, transactions, nil
}

func formatTransactions(w io.Writer, preamble, transactions []string) error {
	// run `hledger print` to format the transactions in ledgerFile
	// and write them to tmpfile
	cmd := exec.Command("hledger", "-f", "-", "print")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	cmd.Stdout = w

	go func() {
		defer stdin.Close()
		// rewrite preamble to comment out include statements and transactions
		inTransaction := false
		const comment = "; "
		for _, line := range preamble {
			if reInclude.MatchString(line) {
				fmt.Fprintln(stdin, comment+line)
				continue
			}

			if reTransaction.MatchString(line) {
				inTransaction = true
				fmt.Fprintln(stdin, comment+line)
				continue
			}

			if inTransaction && rePosting.MatchString(line) {
				fmt.Fprintln(stdin, comment+line)
				continue
			}

			if strings.TrimSpace(line) == "" {
				inTransaction = false
			}

			fmt.Fprintln(stdin, line)
		}
		fmt.Fprintln(stdin, sep)
		for _, line := range transactions {
			fmt.Fprintln(stdin, line)
		}
	}()

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func run(ledgerFile string) error {
	// read the journal - the lines to keep unchanged
	journal, err := os.Open(ledgerFile)
	if err != nil {
		return err
	}

	preamble, transactions, err := parseJournal(journal)
	journal.Close()
	if err != nil {
		return err
	}

	// Create tempfile - write the formatted hledger journal here
	tmpfile, err := ioutil.TempFile(
		filepath.Dir(ledgerFile),
		filepath.Base(ledgerFile)+".tmp_")
	if err != nil {
		return err
	}

	// write preamble as is to tmpfile
	fmt.Fprintln(tmpfile, strings.Join(preamble, "\n"))
	fmt.Fprintln(tmpfile, sep)
	fmt.Fprintln(tmpfile)

	if err := formatTransactions(tmpfile, preamble, transactions); err != nil {
		tmpfile.Close()
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}

	// Replace ledgerFile with the newly formatted tmpfile
	if err := os.Rename(tmpfile.Name(), ledgerFile); err != nil {
		return err
	}

	return nil
}

func main() {
	ledgerFile := os.Getenv("LEDGER_FILE")
	if ledgerFile == "" {
		ledgerFile = filepath.Join(os.Getenv("HOME"), ".hledger.journal")
	}

	flag.StringVar(&ledgerFile, "f", ledgerFile, "hledger journal file")
	flag.Parse()

	if err := run(ledgerFile); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
