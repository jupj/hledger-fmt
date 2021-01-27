package main

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {

	preamble, transactions, err := parseJournal(strings.NewReader(`
D 10,00 €

2021-01-01 Pre-transaction
    expense          7,90
    income                -7,90

; :::Transactions:::

2021-01-03 Prisma
    expense           135,43 €
    income       -135,43
`))

	if err != nil {
		t.Fatal(err)
	}

	const expectedPreamble = `
D 10,00 €

2021-01-01 Pre-transaction
    expense          7,90
    income                -7,90
`

	got := strings.Join(preamble, "\n")
	if got != expectedPreamble {
		t.Errorf("Got preamble:\n%q\nExpected:\n%q\n", got, expectedPreamble)
	}

	const expectedTransactions = `
2021-01-03 Prisma
    expense           135,43 €
    income       -135,43`
	got = strings.Join(transactions, "\n")
	if got != expectedTransactions {
		t.Errorf("Got transactionis:\n%q\nExpected:\n%q\n", got, expectedTransactions)
	}

}

func TestFormat(t *testing.T) {

	preamble, transactions, err := parseJournal(strings.NewReader(`
D 10,00 €

2021-01-01 Pre-transaction
    expense          7,90
    income                -7,90

; :::Transactions:::

2021-01-03 Prisma
    expense           135,43 €
    income       -135,43
`))

	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	err = formatTransactions(&buf, preamble, transactions)
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	const expected = `2021-01-03 Prisma
    expense        135,43 €
    income        -135,43 €

`
	if got != expected {
		t.Errorf("Got journal:\n%q\nExpected:\n%q\n", got, expected)
	}

}
