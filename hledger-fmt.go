package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
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

// readPreamble returns the preamble for ledgerFile
// return an error if
// - ledgerFile has no separator line
// - ledgerFile has more than one separator line
// - lines below the separator line are anything else than transactions
func readPreamble(ledgerFile string) ([]string, error) {
	f, err := os.Open(ledgerFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read lines up to sep
	scan := bufio.NewScanner(f)
	var preamble []string
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
		return nil, errors.New("ledger file contains no transaction separator")
	}

	// Check that anything after this should be a valid transaction
	for scan.Scan() {
		lineNr++
		if scan.Text() == sep {
			return nil, errors.New("ledger file contains multiple transaction separators")
		}

		// Allow empty lines
		if strings.TrimSpace(scan.Text()) == "" {
			continue
		}

		// Date - starts transaction
		if regexp.MustCompile(`^20\d\d-\d\d-\d\d `).Match(scan.Bytes()) {
			continue
		}

		// posting lines - must be indented
		if regexp.MustCompile(`^\s+\S`).Match(scan.Bytes()) {
			continue
		}

		return nil, fmt.Errorf("ledger file contains unexpected line %d in transactions:\n%s", lineNr, scan.Text())
	}

	if err := scan.Err(); err != nil {
		return nil, scan.Err()
	}
	return preamble, nil
}

func run(ledgerFile string) error {
	// Return an error if ledgerFile doesn't exist
	if _, err := os.Stat(ledgerFile); err != nil {
		return err
	}

	// Create tempfile - write the formatted hledger journal here
	tmpfile, err := ioutil.TempFile(
		filepath.Dir(ledgerFile),
		filepath.Base(ledgerFile)+".tmp_")
	if err != nil {
		return err
	}

	// read the preable - the lines to keep unchanged
	preamble, err := readPreamble(ledgerFile)
	if err != nil {
		return err
	}

	// write preamble to tmpfile
	fmt.Fprintln(tmpfile, strings.Join(preamble, "\n"))
	fmt.Fprintln(tmpfile, sep)
	fmt.Fprintln(tmpfile)

	// run `hledger print` to format the transactions in ledgerFile
	// and write them to tmpfile
	cmd := exec.Command("hledger", "-f", ledgerFile, "print")
	cmd.Stderr = os.Stderr
	cmd.Stdout = tmpfile
	if err := cmd.Run(); err != nil {
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
