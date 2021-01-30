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

// formatTransactions reads a hledger journal from r, formats all transactions
// under the separator line, and writes the formatted journal to w
func formatTransactions(w io.Writer, r io.Reader) error {
	preamble, transactions, err := parseJournal(r)
	if err != nil {
		return err
	}

	// Write preamble "as is" to w
	fmt.Fprintln(w, strings.Join(preamble, "\n"))
	fmt.Fprintln(w, sep)
	fmt.Fprintln(w)

	// run `hledger print` to format the transactions in ledgerFile
	cmd := exec.Command("hledger", "-f", "-", "--ignore-assertions", "print")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr

	go func() {
		defer stdin.Close()
		// rewrite preamble to comment out include statements and transactions
		inTransaction := false
		const comment = "; "
		for _, line := range preamble {
			switch {
			case strings.TrimSpace(line) == "":
				inTransaction = false
			case reTransaction.MatchString(line):
				inTransaction = true
				fallthrough
			case reInclude.MatchString(line), inTransaction && rePosting.MatchString(line):
				fmt.Fprint(stdin, comment)
			}

			fmt.Fprintln(stdin, line)
		}
		fmt.Fprintln(stdin, sep)
		for _, line := range transactions {
			fmt.Fprintln(stdin, line)
		}
	}()

	// Get formatted transactions
	journal, err := cmd.Output()
	if err != nil {
		return err
	}

	// Remove trailing empty lines from transactions
	journal = regexp.MustCompile(`\n+$`).ReplaceAll(journal, []byte("\n"))

	// Write transactions to w
	if _, err := w.Write(journal); err != nil {
		return err
	}

	return nil
}

func run(ledgerFile string) error {
	// read the journal file to format
	journal, err := os.Open(ledgerFile)
	if err != nil {
		return err
	}

	// Create tempfile - write the formatted journal here
	tmpfile, err := ioutil.TempFile(
		filepath.Dir(ledgerFile),
		filepath.Base(ledgerFile)+".tmp_")
	if err != nil {
		return err
	}

	// Format journal to tmpfile
	if err := formatTransactions(tmpfile, journal); err != nil {
		tmpfile.Close()
		journal.Close()
		return err
	}

	// Close files, return error if any close fails
	if err := tmpfile.Close(); err != nil {
		journal.Close()
		return err
	}
	if err := journal.Close(); err != nil {
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
