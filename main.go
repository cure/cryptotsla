package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	coinbasepro "github.com/preichenberger/go-coinbasepro/v2"
	"github.com/prometheus/client_golang/prometheus"
	//"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shopspring/decimal"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

var (
	// Debug output goes nowhere by default
	debug = func(string, ...interface{}) {}
	// Set up a *log.Logger for debug output
	debugLog     = log.New(os.Stderr, "DEBUG: ", log.LstdFlags)
	models       map[string]Model
	version      = "dev"
	spotChannels = make(map[string](chan decimal.Decimal)) // receive spot price
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

var httpReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests processed by HTTP status code and method.",
	},
	[]string{"code", "method"},
)

var modelReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tsla_model_total",
		Help: "Total number of successful requests by vehicle model.",
	},
	[]string{"model"},
)

var variantReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tsla_model_variant_total",
		Help: "Total number of successful requests by vehicle model and variant.",
	},
	[]string{"model", "variant"},
)

var optionReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tsla_model_variant_option_total",
		Help: "Total number of successful requests by vehicle model, variant and option.",
	},
	[]string{"model", "variant", "option"},
)

var optionsReqs = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tsla_model_variant_options_total",
		Help: "Total number of successful requests by vehicle model, variant and all valid options.",
	},
	[]string{"model", "variant", "options"},
)

func prometheusRecordOptions(model, variant string, response Response) {
	options := make([]string, 0, len(response.Options))
	for o := range response.Options {
		options = append(options, normalize(o))
	}

	sort.Strings(options)

	optionsReqs.WithLabelValues(strings.ToUpper(model), normalize(variant), strings.Join(options, ",")).Inc()
}

func normalize(input string) (output string) {
	if strings.ToLower(input) == "standardrange" {
		output = "StandardRange"
	} else if strings.ToLower(input) == "standardrangeplus" {
		output = "StandardRangePlus"
	} else if strings.ToLower(input) == "longrange" {
		output = "LongRange"
	} else if strings.ToLower(input) == "plaidplus" {
		output = "PlaidPlus"
	} else if strings.ToLower(input) == "destinationfee" {
		output = "DestinationFee"
	} else if strings.ToLower(input) == "towhitch" {
		output = "TowHitch"
	} else if strings.ToLower(input) == "arachnidwheels" {
		output = "ArachnidWheels"
	} else if strings.ToLower(input) == "sportwheels" {
		output = "SportWheels"
	} else if strings.ToLower(input) == "turbinewheels" {
		output = "TurbineWheels"
	} else if strings.ToLower(input) == "inductionwheels" {
		output = "InductionWheels"
	} else if strings.ToLower(input) == "blackandwhiteinterior" {
		output = "BlackAndWhiteInterior"
	} else if strings.ToLower(input) == "creaminterior" {
		output = "CreamInterior"
	} else if strings.ToLower(input) == "sixseatinterior" {
		output = "SixSeatInterior"
	} else if strings.ToLower(input) == "sevenseatinterior" {
		output = "SevenSeatInterior"
	} else if strings.ToLower(input) == "enhancedautopilot" {
		output = "EnhancedAutopilot"
	} else if strings.ToLower(input) == "fsd" {
		output = "FSD"
	} else {
		output = strings.Title(input)
	}
	return output
}

func loadConfigDefaults() {
	viper.SetDefault("Port", 8080)
	viper.SetDefault("BasePath", "/")
	viper.SetDefault("ListenHost", "127.0.0.1")
	// The header names are camel cased: IP becomes Ip
	viper.SetDefault("ClientIPHeader", "")
	viper.SetDefault("ReferralURL", "")
	viper.SetDefault("GeneratedByURL", "https://api.cryptotsla.com")
	viper.SetDefault("Debug", false)
	viper.SetDefault("Version", false)
	viper.SetDefault("SpotRefreshSeconds", 10)
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
		debug(readableViperSettings())
	}

	if viper.GetBool("Version") {
		fmt.Println(version)
		os.Exit(0)
	}
}

func readableViperSettings() string {
	c := viper.AllSettings()
	settings, err := yaml.Marshal(c)
	if err != nil {
		log.Fatalf("Unable to marshal config to YAML: %v", err)
	}
	return string(settings)
}

func createSpotListeners() {
	currencies := getCurrencies()

	for currency := range currencies {
		spotChannels[currency] = make(chan decimal.Decimal)
		go listen(currency)
	}
}

func getSpot(client *coinbasepro.Client, currency string) (spot decimal.Decimal) {
	book, err := client.GetBook("BTC-"+currency, 1)
	if err != nil {
		fmt.Printf("WARNING: Unable to get the BTC exchange rate for %s: %s\n", currency, err.Error())
		return decimal.New(0, 0)
	}
	spot, err = decimal.NewFromString(book.Bids[0].Price)
	if err != nil {
		fmt.Printf("ERROR: unable to convert the BTC exchange rage to a decimal.Decimal: %s\n", err.Error())
		return decimal.New(0, 0)
	}
	debug("New BTC price for %s: %+v\n", currency, spot)
	return
}

func listen(currency string) {
	client := coinbasepro.NewClient()
	lastPrice := getSpot(client, currency)
	enabled := true
	if lastPrice.IsZero() {
		enabled = false
	}
	for {
		select {
		case spotChannels[currency] <- lastPrice:
			lastPrice = getSpot(client, currency)
		case <-time.After(time.Duration(viper.GetInt("SpotRefreshSeconds")) * time.Second):
			if enabled {
				debug("Timer tick for %s\n", currency)
				lastPrice = getSpot(client, currency)
			} else {
				debug("Timer tick for %s (disabled, not getting new BTC price)\n", currency)
			}
		}
	}
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

func getCurrencies() (currencies map[string]bool) {
	currencies = make(map[string]bool)
	for _, m := range models {
		for _, v := range m.Variants {
			for currency := range v {
				currencies[strings.ToUpper(currency)] = true
			}
		}
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
			model.Options = append(model.Options, normalize(option))
		}
		for variant := range m.Variants {
			model.Variants = append(model.Variants, normalize(variant))
		}
		response.Models = append(response.Models, model)
	}
	responseB, err := json.Marshal(response)
	if err != nil {
		log.Print("Unable marshal response to JSON", err.Error())
	}

	fmt.Fprintf(w, "%s", string(responseB))
	accessLog(r, "200", string(responseB))
}

func accessLog(r *http.Request, code, message string) {
	remote := r.RemoteAddr
	if realIP, ok := r.Header[viper.GetString("ClientIPHeader")]; ok {
		remote = strings.Join(realIP, ",")
	}
	log.Printf("{ \"Remote\":\"%s\", \"Request\": \"%s %s\", \"Response\": %s}\n", remote, r.Method, r.URL.String(), message)
	httpReqs.WithLabelValues(code, r.Method).Inc()
}

func getHelp(w http.ResponseWriter, r *http.Request) {
	if r.URL.String() == "/" {
		response := "{\"Status\":\"200\",\"Message\":\"See https://github.com/cure/cryptotsla for documentation\"}"
		fmt.Fprintf(w, "%s", response)
		accessLog(r, "200", response)
		return
	}
	response := "{\"Status\":\"404\",\"Error\":\"Path not found, see https://github.com/cure/cryptotsla\"}"
	httpReqs.WithLabelValues("404", r.Method).Inc()
	accessLog(r, "404", response)
	http.Error(w, response, http.StatusNotFound)
}

func getModel(w http.ResponseWriter, r *http.Request) {
	urlPart := strings.Split(r.URL.Path, "/")

	if len(urlPart) < 3 {
		errorString := "{\"Status\":\"404\",\"Error\":\"Path not found, see https://github.com/cure/cryptotsla\"}"
		accessLog(r, "404", errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	model := normalize(urlPart[2])
	modelReqs.WithLabelValues(strings.ToUpper(model)).Inc()

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

	response.BTCSpotPrice = <-spotChannels[response.Currency]

	if response.BTCSpotPrice.IsZero() {
		errorString := "{\"Status\":\"500\",\"Error\":\"Unable to get BTC exchange rate\"}"
		accessLog(r, "500", errorString)
		http.Error(w, errorString, http.StatusInternalServerError)
		return
	}

	m, ok := models[model]
	if !ok {
		errorString := "{\"Status\":\"404\",\"Error\":\"Model not found\"}"
		accessLog(r, "404", errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	// Apply DefaultVariant for the model, if necessary
	if variant == "" {
		variant = m.DefaultVariant
	}
	response.Variant = normalize(variant)
	variant = strings.ToLower(variant)

	variantReqs.WithLabelValues(strings.ToUpper(model), normalize(variant)).Inc()

	_, ok = m.Variants[variant]
	if !ok {
		errorString := "{\"Status\":\"404\",\"Error\":\"Variant not found\"}"
		accessLog(r, "404", errorString)
		http.Error(w, errorString, http.StatusNotFound)
		return
	}

	cost, ok := m.Variants[variant][strings.ToLower(currency)]
	if !ok {
		errorString := "{\"Status\":\"404\",\"Error\":\"Currency not available for this model/variant\"}"
		accessLog(r, "404", errorString)
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
			response.Options[normalize(v)] = cost
			response.Total += cost
			optionReqs.WithLabelValues(strings.ToUpper(model), normalize(variant), normalize(v)).Inc()
		}
	}
	prometheusRecordOptions(model, variant, response)
	destinationFee := m.Options["destinationfee"][currency]
	if destinationFee != 0 {
		total += destinationFee
		response.Options["DestinationFee"] = destinationFee
		response.Total += destinationFee
		optionReqs.WithLabelValues(strings.ToUpper(model), normalize(variant), "DestinationFee").Inc()
	}

	t := time.Now()
	response.Timestamp = t
	response.TotalBTC = decimal.NewFromFloat(float64(total)).Div(response.BTCSpotPrice)

	responseB, err := json.Marshal(response)
	if err != nil {
		log.Print("Unable to marshal response to JSON", err.Error())
	}

	fmt.Fprintf(w, "%s", string(responseB))
	accessLog(r, "200", string(responseB))
}

func main() {
	flags := parseFlags()
	loadConfig(flags)

	err := viper.UnmarshalKey("Models", &models)
	if err != nil {
		log.Fatal("Unable to unmarshal Models", err.Error())
	}
	// debug("Models: %+v\n", models)

	createSpotListeners()

	log.Println("Starting cryptotsla Daemon")

	prometheus.MustRegister(httpReqs)
	prometheus.MustRegister(modelReqs)
	prometheus.MustRegister(variantReqs)
	prometheus.MustRegister(optionReqs)
	prometheus.MustRegister(optionsReqs)

	http.Handle(viper.GetString("BasePath"), http.HandlerFunc(getHelp))
	http.Handle(viper.GetString("BasePath")+"model/", http.HandlerFunc(getModel))
	http.Handle(viper.GetString("BasePath")+"model", http.HandlerFunc(getModel))
	http.Handle(viper.GetString("BasePath")+"available/", http.HandlerFunc(listModels))
	http.Handle(viper.GetString("BasePath")+"available", http.HandlerFunc(listModels))

	http.Handle("/metrics", promhttp.Handler())
	debug("Listening on %s:%s", viper.GetString("ListenHost"), viper.GetString("Port"))
	log.Fatal(http.ListenAndServe(viper.GetString("ListenHost")+":"+viper.GetString("Port"), nil))
}
