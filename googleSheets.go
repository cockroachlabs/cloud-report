package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	sheets "google.golang.org/api/sheets/v4"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getSheetsClient() *sheets.Service {
	b, err := ioutil.ReadFile("./credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file; must be in same dir as binary %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	return srv
}

// ssID is the ID of the spreadsheet where you want to log results.
const ssID = "175Q3g3Ti40rEmaCMwm0CBXRnHIo07WSBsxOVhmqNyEY"

// filenameToSSRange must be manually constructed to map the
// names of the files you might read to the sheets and ranges
// they should be inserted into in the sheet identified using
// the ssID const.
var filenameToSSRange = map[string]string{
	"/cpu.csv":                  "CPU!A:G",
	"/io-load-results.csv":      "'FileIO Load'!A:B",
	"/io-rd-results.csv":        "'FileIO Read'!A:I",
	"/io-wr-results.csv":        "'FileIO Write'!A:J",
	"/network-iperf-client.csv": "iperf!A:D",
	"/network-ping.csv":         "ping!A:E",
	"/run-data.csv":             "Runs!A:E",
}

// appendDataToSpreadsheet inserts the CSV file in the named directory
// into the spreadsheet identified using the ssID const.
func appendDataToSpreadsheet(fileName, dir string) {

	ssRange, ok := filenameToSSRange[fileName]

	if !ok {
		log.Printf("No filename %s; cannot log to spreadsheet\n", fileName)
		return
	}

	file, err := os.Open(dir + fileName)
	if err != nil {
		log.Printf("Unable to open file: %v\n", err)
		return
	}

	r := csv.NewReader(file)

	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	// Remove header row from CSV.
	recordsWOHeader := records[1:]

	// Convert recordsWOHeader to 2D interface slice.
	s := make([][]interface{}, len(recordsWOHeader))
	for i, v := range recordsWOHeader {
		s[i] = make([]interface{}, len(v))
		for j, w := range v {
			s[i][j] = w
		}
	}

	srv := getSheetsClient()

	vr := sheets.ValueRange{
		MajorDimension: "ROWS",
		Values:         s,
	}

	_, err = srv.Spreadsheets.Values.Append(ssID, ssRange, &vr).ValueInputOption("RAW").Do()

	if err != nil {
		log.Fatal(err)
	}
}
