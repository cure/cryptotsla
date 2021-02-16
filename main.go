package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	coinbasepro "github.com/preichenberger/go-coinbasepro/v2"
	"github.com/shopspring/decimal"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// Debug output goes nowhere by default
	debug = func(string, ...interface{}) {}
	// Set up a *log.Logger for debug output
	debugLog = log.New(os.Stderr, "DEBUG: ", log.LstdFlags)
	models   map[string]Model
	version  = "dev"
)

// Model describes a vehicle model
type Model struct {
	DefaultVariant string                      `mapstructure:"DefaultVariant"`
	Options        map[string]map[string]int64 `mapstructure:"Options"`
	Variants       map[string]map[string]int64 `mapstructure:"Variants"`
}

// Response describes the response to a request
type Response struct {
	Model          string
	BasePrice      int64
	Variant        string
	Options        map[string]int64
	BTCSpotPrice   decimal.Decimal
	Currency       string
	Total          int64
	TotalBTC       decimal.Decimal
	ReferralURL    string
	GeneratedByURL string
	Timestamp      time.Time
}

// AvailableModel describes an available vehicle model
type AvailableModel struct {
	Name     string
	Options  []string
	Variants []string
}

// AvailableResponse describes all available vehicle models
type AvailableResponse struct {
	Models []AvailableModel
}

func loadConfigDefaults() {
	viper.SetDefault("Port", 8080)
	viper.SetDefault("BasePath", "/")
	viper.SetDefault("ListenHost", "127.0.0.1")
	viper.SetDefault("ReferralURL", "")
	viper.SetDefault("GeneratedByURL", "https://api.cryptotsla.com")
	viper.SetDefault("Debug", false)
	viper.SetDefault("Version", false)
}

func loadConfig(flags *flag.FlagSet) {
	loadConfigDefaults()

	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/cryptotsla/")
	viper.AddConfigPath("$HOME/.cryptotsla")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		usage(flags)
		log.Fatal("Fatal error reading config file:", err.Error())
	}

	if viper.GetBool("Debug") {
		debug = debugLog.Printf
	}

	if viper.GetBool("Version") {
		fmt.Println(version)
		os.Exit(0)
	}

}

func getBtcSpot(client *coinbasepro.Client, currency string) (price decimal.Decimal) {
	book, err := client.GetBook("BTC-"+currency, 1)
	if err != nil {
		log.Print(err.Error())
		return decimal.New(0, 0)
	}

	lastPrice, err := decimal.NewFromString(book.Bids[0].Price)
	if err != nil {
		println(err.Error())
	}
	return lastPrice
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return ""
}

func (i *arrayFlags) Set(value string) error {
	for _, s := range strings.Split(value, ",") {
		*i = append(*i, s)
	}
	return nil
}

func parseFlags() (flags *flag.FlagSet) {
	flags = flag.NewFlagSet("cryptotsla", flag.ExitOnError)
	flags.Usage = func() { usage(flags) }

	flags.Bool("version", false, "print version and exit")
	flags.Bool("debug", false, "enable debug logging")

	// Parse args; omit the first arg which is the command name
	err := flags.Parse(os.Args[1:])
	if err != nil {
		log.Fatal("Unable to parse command line arguments:", err.Error())
	}

	err = viper.BindPFlags(flags)
	if err != nil {
		log.Fatal(err)
	}

	return
}

func listModels(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s: %s %s\n", r.RemoteAddr, r.Method, r.URL.String())

	var response AvailableResponse

	for name, m := range models {
		var model AvailableModel
		model.Name = strings.ToUpper(name)
		for option := range m.Options {
			model.Options = append(model.Options, option)
		}
		for variant := range m.Variants {
			model.Variants = append(model.Variants, variant)
		}
		response.Models = append(response.Models, model)
	}
	responseB, err := json.Marshal(response)
	if err != nil {
		log.Print("Unable marshal response to JSON", err.Error())
	}

	fmt.Fprintf(w, "%s", string(responseB))
	accessLog(r, string(responseB))
}

func accessLog(r *http.Request, message string) {
	remote := r.RemoteAddr
	// Oddly, the header has been changed from X-Real-IP
	if realIP, ok := r.Header["X-Real-Ip"]; ok {
		remote = strings.Join(realIP, ",")
	}
	log.Printf("{ \"Remote\":\"%s\", \"Request\": \"%s %s\", \"Response\": %s}\n", remote, r.Method, r.URL.String(), message)
}

func getHelp(w http.ResponseWriter, r *http.Request) {
	if r.URL.String() == "/" {
		response := "{\"Status\":\"200\",\"Message\":\"See https://github.com/cure/cryptotsla for documentation\"}"
		fmt.Fprintf(w, "%s", response)
		accessLog(r, response)
		return
	}
	response := "{\"Status\":\"404\",\"Error\":\"Path not found, see https://github.com/cure/cryptotsla\"}"
	http.Error(w, response, http.StatusNotFound)
}

func getModel(w http.ResponseWriter, r *http.Request) {
	urlPart := strings.Split(r.URL.Path, "/")

	if len(urlPart) < 3 {
		errorString := "{\"Status\":\"404\",\"Error\":\"Path not found, see https://github.com/cure/cryptotsla\"}"
		accessLog(r, errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	model := urlPart[2]

	var variant string
	if len(urlPart) > 3 {
		variant = urlPart[3]
	}

	params := r.URL.Query()

	currency, ok := params["currency"]
	if !ok {
		currency = []string{"USD"}
	}

	var options arrayFlags = strings.Split(strings.Join(params["options"], ""), ",")

	generateResponse(w, r, strings.ToLower(strings.Join(currency, "")), model, variant, options)
}

func generateResponse(w http.ResponseWriter, r *http.Request, currency, model, variant string, options arrayFlags) {

	var m Model
	var total int64

	var response Response
	response.Model = model
	response.ReferralURL = viper.GetString("ReferralURL")
	response.GeneratedByURL = viper.GetString("GeneratedByURL")
	response.Currency = strings.ToUpper(currency)

	model = strings.ToLower(model)

	client := coinbasepro.NewClient()
	response.BTCSpotPrice = getBtcSpot(client, response.Currency)

	if response.BTCSpotPrice.IsZero() {
		http.Error(w, "{\"Status\":\"500\",\"Error\":\"Unable to get BTC exchange rate\"}", http.StatusInternalServerError)
		return
	}

	m, ok := models[model]
	if !ok {
		http.Error(w, "{\"Status\":\"404\",\"Error\":\"Model not found\"}", http.StatusNotFound)
		return
	}

	// Apply DefaultVariant for the model, if necessary
	if variant == "" {
		variant = m.DefaultVariant
	}
	response.Variant = variant
	variant = strings.ToLower(variant)

	_, ok = m.Variants[variant]
	if !ok {
		errorString := "{\"Status\":\"404\",\"Error\":\"Variant not found\"}"
		accessLog(r, errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	cost, ok := m.Variants[variant][strings.ToLower(currency)]
	if !ok {
		errorString := "{\"Status\":\"404\",\"Error\":\"Currency not available for this model/variant\"}"
		accessLog(r, errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	total += cost
	response.BasePrice = cost
	response.Total += cost

	response.Options = make(map[string]int64)

	var groups map[int64]bool = make(map[int64]bool)
	for _, v := range options {
		cost := m.Options[strings.ToLower(v)][currency]
		if group, ok := m.Options[strings.ToLower(v)]["group"]; ok {
			if _, found := groups[group]; found {
				// We've already selected an option from this group, skip this option
				continue
			}
			groups[group] = true
		}
		// Only include the option if we have cost information about it, ignore otherwise
		if cost != int64(0) {
			total += cost
			response.Options[v] = cost
			response.Total += cost
		}
	}
	destinationFee := m.Options["destinationfee"][currency]
	if destinationFee != 0 {
		total += destinationFee
		response.Options["DestinationFee"] = destinationFee
		response.Total += destinationFee
	}

	t := time.Now()
	response.Timestamp = t
	response.TotalBTC = decimal.NewFromFloat(float64(total)).Div(response.BTCSpotPrice)

	responseB, err := json.Marshal(response)
	if err != nil {
		log.Print("Unable to marshal response to JSON", err.Error())
	}

	fmt.Fprintf(w, "%s", string(responseB))
	accessLog(r, string(responseB))
}

func main() {
	flags := parseFlags()
	loadConfig(flags)

	err := viper.UnmarshalKey("Models", &models)
	if err != nil {
		log.Fatal("Unable to unmarshal Models", err.Error())
	}
	debug("Models: %+v\n", models)

	log.Println("Starting cryptotsla Daemon")

	http.Handle(viper.GetString("BasePath"), http.HandlerFunc(getHelp))
	http.Handle(viper.GetString("BasePath")+"model/", http.HandlerFunc(getModel))
	http.Handle(viper.GetString("BasePath")+"model", http.HandlerFunc(getModel))
	http.Handle(viper.GetString("BasePath")+"available/", http.HandlerFunc(listModels))
	http.Handle(viper.GetString("BasePath")+"available", http.HandlerFunc(listModels))

	log.Fatal(http.ListenAndServe(viper.GetString("ListenHost")+":"+viper.GetString("Port"), nil))
}
