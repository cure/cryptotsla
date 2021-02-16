
## Overview

`cryptotsla` calculates the current Bitcoin spot price of a specific Tesla vehicle configuration, optionally taking a currency and regionally available Tesla vehicle configurations into account.

The information is made available via a JSON API that can be queried via http, e.g.:

```
curl -s "https://api.cryptotsla.com/model/Y" |jq .
{
  "Model": "Y",
  "BasePrice": 41990,
  "Variant": "StandardRange",
  "Options": {
    "DestinationFee": 1200
  },
  "BTCSpotPrice": "48026.86",
  "Currency": "USD",
  "Total": 43190,
  "TotalBTC": "0.8992884398438707",
  "GeneratedByURL": "https://api.cryptotsla.com",
  "Timestamp": "2021-02-15T17:28:59.903489389-05:00"
}
```

To get just the BTC total:

```
curl -s "https://api.cryptotsla.com/model/Y"  |jq .TotalBTC
0.8992884398438707
```

The default model Y variant, "StandardRange", was selected automatically. To request a different variant:

```
curl -s "https://api.cryptotsla.com/model/Y/LongRange" |jq .
{
  "Model": "Y",
  "BasePrice": 49990,
  "Variant": "LongRange",
  "Options": {
    "DestinationFee": 1200
  },
  "BTCSpotPrice": "48192.08",
  "Currency": "USD",
  "Total": 51190,
  "TotalBTC": "1.0622077320588777",
  "GeneratedByURL": "https://api.cryptotsla.com",
  "Timestamp": "2021-02-15T17:36:50.646497513-05:00"
}
```

It is also possible to specify vehicle options:

```
curl -s "https://api.cryptotsla.com/model/Y/LongRange?options=TowHitch,SevenSeatInterior" |jq .
{
  "Model": "Y",
  "BasePrice": 49990,
  "Variant": "LongRange",
  "Options": {
    "DestinationFee": 1200,
    "SevenSeatInterior": 3000,
    "TowHitch": 1000
  },
  "BTCSpotPrice": "48108.21",
  "Currency": "USD",
  "Total": 55190,
  "TotalBTC": "1.147205435413207",
  "GeneratedByURL": "https://api.cryptotsla.com",
  "Timestamp": "2021-02-15T17:37:36.980907053-05:00"
}
```

If an option is requested that does not exist, it will be ignored. Some options only exist in certain territories and will only be included when the relevant currency is selected.

A different currency can be selected. This also limits the response to the models, variants and options available in the relevant territory, for example:

```
curl -s "https://api.cryptotsla.com/model/3/LongRange?options=Red&currency=GBP" |jq .
{
  "Model": "3",
  "BasePrice": 46990,
  "Variant": "LongRange",
  "Options": {
    "Red": 2000
  },
  "BTCSpotPrice": "34697.26",
  "Currency": "GBP",
  "Total": 48990,
  "TotalBTC": "1.4119270513003044",
  "GeneratedByURL": "https://api.cryptotsla.com",
  "Timestamp": "2021-02-15T17:50:49.112942967-05:00"
}
```

Model, variant and option names are case insensitive.

Get a list of available models and options:

```
curl -s "https://api.cryptotsla.com/available" |jq .
{
  "Models": [
    {
      "Name": "Y",
      "Options": [
        "inductionwheels",
        "destinationfee",
        "black",
        "blackandwhiteinterior",
        "fsd",
        "sevenseatinterior",
        "blue",
        "towhitch",
        "silver",
        "red"
      ],
      "Variants": [
        "standardrange",
        "longrange",
        "performance"
      ]
    },
    {
      "Name": "S",
      "Options": [
        "destinationfee",
        "enhancedautopilot",
        "black",
        "blue",
        "blackandwhiteinterior",
        "arachnidwheels",
        "fsd",
        "silver",
        "red",
        "creaminterior"
      ],
      "Variants": [
        "longrange",
        "plaid",
        "plaidplus"
      ]
    },
    {
      "Name": "3",
      "Options": [
        "enhancedautopilot",
        "blackandwhiteinterior",
        "blue",
        "red",
        "fsd",
        "black",
        "towhitch",
        "sportwheels",
        "wintertires",
        "silver",
        "destinationfee"
      ],
      "Variants": [
        "standardrangeplus",
        "longrange",
        "performance"
      ]
    },
    {
      "Name": "X",
      "Options": [
        "destinationfee",
        "red",
        "turbinewheels",
        "silver",
        "enhancedautopilot",
        "creaminterior",
        "blue",
        "black",
        "blackandwhiteinterior",
        "sixseatinterior",
        "sevenseatinterior",
        "fsd"
      ],
      "Variants": [
        "longrange",
        "plaid"
      ]
    }
  ]
}
```

## Room for improvement

* There is no BTC-CAD order book via this Coinbase API. Use a different API? The Coinbase developer API seems to have a price endpoint for CAD.
* We need some sort of locale option. Particularly for the Euro pricing, where there are differences between the countries that use the Euro.
* Many currencies are missing from `config.yml`
* Add tests

## Building the software

GNU Make and a Go compiler, version 1.11 or higher are required.

Build  with the *make* command from the root of the tree.

## Running the software

Copy the `config.yml.example` file to `config.yml` and adjust as necessary. Run the software with the `./cryptotsla` command. A sample systemd unit file is provided in the `systemd` subdirectory.

## Hacking

GNU Make and a Go compiler, version 1.11 or higher are required. In addition,
[GolangCI-Lint](https://github.com/golangci/golangci-lint) is needed.

Build the software with the *make dev* command.

## Licensing

`cryptotsla` is Free Software, released under the GNU Affero GPL v3 or later. See the LICENSE file for the text of the license.
