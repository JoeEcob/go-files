package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mailgun/mailgun-go"
	qrcode "github.com/skip2/go-qrcode"
)

var (
	configFile      = flag.String("config", "config.json", "Path to the configuration file")
	octopusAPIKey   string
	octopusAPIToken string
	mailgunDomain   string
	mailgunApiKey   string
	mailgunFrom     string
	mailgunTo       string
)

type Config struct {
	OctopusAPIKey string `json:"octopusAPIKey"`
	MailgunDomain string `json:"mailgunDomain"`
	MailgunApiKey string `json:"mailgunApiKey"`
	MailgunFrom   string `json:"mailgunFrom"`
	MailgunTo     string `json:"mailgunTo"`
}

type TokenResponse struct {
	Data struct {
		ObtainKrakenToken map[string]interface{} `json:"obtainKrakenToken"`
	} `json:"data"`
}

type RewardResponse struct {
	Data struct {
		OctoplusRewards []OctoplusReward `json:"octoplusRewards"`
	} `json:"data"`
}

type OctoplusReward struct {
	ID       int               `json:"id"`
	PriceTag string            `json:"priceTag"`
	Status   string            `json:"status"`
	Vouchers []OctoplusVoucher `json:"vouchers"`
}

type OctoplusVoucher struct {
	Code          string `json:"code"`
	BarcodeValue  string `json:"barcodeValue"`
	BarcodeFormat string `json:"barcodeFormat"`
	ExpiresAt     string `json:"expiresAt"`
}

func main() {
	// Parse command line flags
	flag.Parse()

	// Set log flags to enable date and time
	log.SetFlags(log.Ldate | log.Ltime)

	// Read configuration file
	config, err := readConfig(*configFile)
	if err != nil {
		log.Fatalf("Error reading configuration: %v", err)
	}

	// Set configuration variables
	octopusAPIKey = config.OctopusAPIKey
	mailgunDomain = config.MailgunDomain
	mailgunApiKey = config.MailgunApiKey
	mailgunFrom = config.MailgunFrom
	mailgunTo = config.MailgunTo

	// Obtain Octopus API token
	err = getOctopusAPIToken()
	if err != nil {
		log.Fatalf("Error obtaining Octopus API token: %v", err)
	}

	// Make Octoplus API request
	reward, err := getOctoplusReward()
	if err != nil {
		log.Fatalf("Error getting Octoplus reward: %v", err)
	}

	// Print Octoplus reward details
	printOctoplusReward(reward)

	// Send the response to Mailgun's Email API
	err = sendToMailgunEmail(reward)
	if err != nil {
		log.Fatalf("Error sending to Mailgun: %v", err)
	}
}

// getOctopusAPIToken obtains an API token for the Octopus Energy API
func getOctopusAPIToken() error {
	url := "https://api.octopus.energy/v1/graphql/"

	// Payload for authentication, adjust based on Octopus Energy API requirements
	payload := strings.NewReader(fmt.Sprintf(`{
		"query": "mutation krakenTokenAuthentication($key: String!) { obtainKrakenToken(input: {APIKey: $key}) { token }}",
		"variables": {
		  "key": "%s"
		}
	  }`, octopusAPIKey))

	// Make HTTP POST request
	resp, err := http.Post(url, "application/json", payload)
	if err != nil {
		return fmt.Errorf("error obtaining Octopus API token: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading Octopus API token response body: %v", err)
	}

	// Unmarshal JSON response
	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return fmt.Errorf("error decoding Octopus API token response JSON: %v", err)
	}

	// Retrieve and store the token
	var ok bool
	octopusAPIToken, ok = tokenResponse.Data.ObtainKrakenToken["token"].(string)
	if !ok {
		return fmt.Errorf("error extracting access_token from Octopus API token response")
	}

	log.Printf("Octopus API token obtained: length %d", len(octopusAPIToken))

	return nil
}

// getOctoplusReward makes an HTTP request to the Octopus Energy API
func getOctoplusReward() (*OctoplusReward, error) {
	url := "https://api.octopus.energy/v1/graphql/"

	// Payload for authentication, adjust based on Octopus Energy API requirements
	payload := strings.NewReader(`{
		"query": "query getOctoplusRewards($rewardId: Int) {\noctoplusRewards(rewardId: $rewardId) {\nid\npriceTag\nstatus\nvouchers {\n ... on OctoplusVoucherType {\ncode\nbarcodeValue\nbarcodeFormat\nexpiresAt}}}}"
	  }`)

	// Make HTTP POST request
	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Add("Authorization", octopusAPIToken)
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making Octoplus API request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading Octopus API response body: %v", err)
	}

	// Unmarshal JSON response
	var rewardResponse RewardResponse
	err = json.Unmarshal(body, &rewardResponse)
	if err != nil {
		return nil, fmt.Errorf("error decoding Octopus API response JSON: %v", err)
	}

	// Check if there are Octoplus rewards
	if len(rewardResponse.Data.OctoplusRewards) == 0 {
		return nil, fmt.Errorf("no Octoplus rewards found in the response")
	}

	// Return the first item, this _should_ be most recent
	return &rewardResponse.Data.OctoplusRewards[0], nil
}

// printOctoplusReward prints Octoplus reward details to the console
func printOctoplusReward(reward *OctoplusReward) {
	log.Printf("Octopus Energy Reward\nID: %d\nPrice Tag: %s\nStatus: %s\n\nVouchers:\n", reward.ID, reward.PriceTag, reward.Status)
	for i, voucher := range reward.Vouchers {
		log.Printf("Voucher %d:\n", i+1)
		log.Printf("  Code: %s\n", voucher.Code)
		log.Printf("  Barcode Value: %s\n", voucher.BarcodeValue)
		log.Printf("  Barcode Format: %s\n", voucher.BarcodeFormat)
		log.Printf("  Expires At: %s\n", voucher.ExpiresAt)
	}
}

// sendToMailgunEmail sends the Octopus Energy response to Twilio's WhatsApp API
func sendToMailgunEmail(reward *OctoplusReward) error {
	// Set up Mailgun client
	mg := mailgun.NewMailgun(mailgunDomain, mailgunApiKey)

	qrCodes := map[string][]byte{}

	// Prepare message body
	messageBody := fmt.Sprintf("Octopus Energy Reward\nID: %d\nPrice Tag: %s\nStatus: %s\n\nVouchers:\n", reward.ID, reward.PriceTag, reward.Status)
	for i, voucher := range reward.Vouchers {
		messageBody += fmt.Sprintf("Voucher %d:\n", i+1)
		messageBody += fmt.Sprintf("  Code: %s\n", voucher.Code)
		messageBody += fmt.Sprintf("  Barcode Value: %s\n", voucher.BarcodeValue)
		messageBody += fmt.Sprintf("  Barcode Format: %s\n", voucher.BarcodeFormat)
		messageBody += fmt.Sprintf("  Expires At: %s\n", voucher.ExpiresAt)

		// Generate QR code from the barcode value
		png, err := qrcode.Encode(voucher.BarcodeValue, qrcode.Medium, 256)
		if err != nil {
			return fmt.Errorf("error generating QR code: %v", err)
		}

		// Add the qrCode to the map, to be attached separately.
		qrCodes[voucher.Code] = png
	}

	// Send email via Mailgun's API
	message := mg.NewMessage(mailgunFrom, "Octopus API - New Reward Generated", messageBody, mailgunTo)

	// Loop through the QR codes and attach each. The name will be the voucher code.
	for k, v := range qrCodes {
		message.AddBufferAttachment(k, v)
	}

	// Send the message with a 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resp, id, err := mg.Send(ctx, message)

	log.Printf("Successfully sent Mailgun email, response: '%s' id: '%s'", resp, id)

	return err
}

// readConfig reads configuration from a JSON file
func readConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening configuration file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, fmt.Errorf("error decoding configuration JSON: %v", err)
	}

	return config, nil
}
