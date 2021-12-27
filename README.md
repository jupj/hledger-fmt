# hledger-fmt

hledger-fmt formats [hledger](https://hledger.org/) journals. It keeps your
journal ordered by date, and nicely formatted, using `hledger print` to format
your transactions.

## Getting started

Prepare the hledger journal file:

1. Add a separator line to the journal file: `; :::Transactions:::`
2. Organize the journal file:
    - anything that is not a transaction above the separator line - this part
      will not be modified by `hledger-fmt`
    - all transactions below the separator line - this part will be formatted
      by `hledger-fmt`

Format your journal:

```
$ hledger-fmt                    # format $LEDGER_FILE
$ hledger-fmt -f 2021.journal    # format 2021.journal
```

## Install as add-on

Install hledger-fmt in your PATH. For example `go install` if you have
`~/go/bin` in your PATH.

Now you can use it as an hledger add-on like so:

`$ hledger fmt`
