package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

var exampleConfigFile = []byte(`
---
BasePath: "/"
Port: 8080
ListenHost: "127.0.0.1"
ReferralURL: "https://ts.la/tom76605"
Models:
  "S":
    DefaultVariant: "LongRange"
    Variants:
      LongRange:
        USD: 79990
        CAD: 114990
        EUR: 89990
        GBP: 83980
    Options:
      DestionationFee:
        USD: 1200
      Red:
        Group: 0
        USD: 2500
        CAD: 3300
        EUR: 2700
        GBP: 2500
`)

func usage(fs *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, `
cryptotsla calculates the current BTC spot price of specific Tesla vehicle configurations.

Config file locations:

  /etc/cryptotsla/config.yml
  ~/.cryptotsla/config.yml
  ./config.yml

Options:
`)
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Example config file:
%s

`, exampleConfigFile)
}
