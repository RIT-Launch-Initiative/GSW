package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const DashboardJsonFile = "data/grafana/Backplane-Live.json"

// Prompts the user for the Grafana username, password, and URL.
// Returns a URL to Grafana containing the username and password info for basic auth.
func getURL(scanner *bufio.Scanner) (*url.URL, error) {
	fmt.Print("Grafana account username: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Grafana account username: %v", err)
	}
	username := scanner.Text()
	fmt.Print("Grafana account password: ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Grafana account username: %v", err)
	}
	password := scanner.Text()
	fmt.Print("Grafana URL (leave blank for 'http://localhost:3000'): ")
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Grafana URL: %v", err)
	}

	urlString := scanner.Text()
	var grafanaURL *url.URL
	if urlString == "" {
		grafanaURL, _ = url.Parse("http://localhost:3000")
	} else {
		var err error
		grafanaURL, err = url.Parse(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("error parsing Grafana URL: %v", err)
		}
	}
	userInfo := url.UserPassword(username, password)
	grafanaURL.User = userInfo
	return grafanaURL, nil
}

// Sends a request to the Grafana API at the specified URL.
// Returns the response body unmarshalled into a map.
func sendRequest(method string, grafanaURL *url.URL, jsonData []byte) (map[string]any, error) {
	requestBody := bytes.NewReader(jsonData)
	request, err := http.NewRequest(method, grafanaURL.String(), requestBody)
	if err != nil {
		return nil, fmt.Errorf("error forming HTTP request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error returned by HTTP request: %v", err)
	}

	responseBodyData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading HTTP response body: %v", err)
	}
	responseBody := make(map[string]any)
	err = json.Unmarshal(responseBodyData, &responseBody)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTTP response JSON: %v", err)
	}
	if response.StatusCode != 200 && response.StatusCode != 201 {
		fmt.Printf("Error: unexpected status code %d\n", response.StatusCode)
		if status, ok := responseBody["status"]; ok {
			return nil, fmt.Errorf("%v (%v)", responseBody["message"], status)
		}
		return nil, fmt.Errorf("%v", responseBody["message"])
	}

	err = response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing response body: %v", err)
	}

	return responseBody, nil
}

// Creates a service account with the admin role at the specified URL.
// Returns the id associated with the created account.
func createAdminServiceAccount(createURL *url.URL, name string) (int, error) {
	jsonData := json.RawMessage(`{"name":"` + name + `", "role":"Admin", "isDisabled":false}`)

	body, err := sendRequest(http.MethodPost, createURL, jsonData)
	if err != nil {
		return -1, err
	}
	idVal, ok := body["id"]
	if !ok {
		return -1, fmt.Errorf("service account id not found")
	}
	id := int(idVal.(float64))

	return id, nil
}

// Adds a token to the service account at the specified URL.
// Returns the generated token key.
func addServiceAccountToken(addTokenURL *url.URL) (string, error) {
	jsonData := json.RawMessage(`{"name":"live_token_` + strconv.Itoa(rand.Int()) + `", "secondsToLive":0}`)

	body, err := sendRequest(http.MethodPost, addTokenURL, jsonData)
	if err != nil {
		return "", err
	}
	keyVal, ok := body["key"]
	if !ok {
		return "", fmt.Errorf("token key not found")
	}
	key := keyVal.(string)

	return key, nil
}

// Writes the token to the .env file in the root directory.
// If an error occurs, or if the user opts not to overwrite an existing file, the token is printed instead.
func setEnvFile(scanner *bufio.Scanner, token string) {
	// .env file does not exist
	if _, err := os.Stat(".env"); errors.Is(err, fs.ErrNotExist) {
		err = os.WriteFile(".env", []byte("GRAFANA_LIVE_TOKEN="+token), 0660)
		if err != nil {
			fmt.Println("Error creating .env file:", err)
			fmt.Println("** Cannot create .env file (you must store the token manually).")
			fmt.Println("** Token: " + token)
		} else {
			fmt.Println(".env file created successfully.")
		}
		return
	}
	// .env file already exists
	fmt.Print("Overwrite existing .env file? (type 'y' or 'yes' to overwrite): ")
	response := ""
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading response:", err)
	} else {
		response = strings.TrimSpace(strings.ToLower(scanner.Text()))
	}
	if response == "y" || response == "yes" {
		err := os.WriteFile(".env", []byte("GRAFANA_LIVE_TOKEN="+token), 0660)
		if err != nil {
			fmt.Println("Error overwriting .env file:", err)
			fmt.Println("** The .env file may not have been overwritten properly (you may need to store the token manually).")
			fmt.Println("** Token: " + token)
		} else {
			fmt.Println(".env file overwritten successfully.")
		}
	} else {
		fmt.Println("** Will not overwrite existing .env file (you must store the token manually).")
		fmt.Println("** Token: " + token)
	}
}

// Creates a dashboard at the specified URL using the specified JSON.
func createDashboard(dashboardURL *url.URL, dashboardJSONFilepath string) error {
	dashboardJSON, err := os.ReadFile(dashboardJSONFilepath)
	if err != nil {
		return fmt.Errorf("error reading dashboard JSON: %v", err)
	}
	dashboardJSONMap := make(map[string]any)
	err = json.Unmarshal(dashboardJSON, &dashboardJSONMap)
	if err != nil {
		return fmt.Errorf("error parsing dashboard JSON: %v", err)
	}
	// set id + uid to null so that a new dashboard is generated
	dashboardTitle := "Backplane-Live-" + strconv.Itoa(rand.Int())
	dashboardJSONMap["id"] = nil
	dashboardJSONMap["uid"] = nil
	dashboardJSONMap["title"] = dashboardTitle

	// construct the full API request data
	requestJSONMap := make(map[string]any)
	requestJSONMap["dashboard"] = dashboardJSONMap
	requestJSONMap["overwrite"] = false
	requestData, err := json.Marshal(requestJSONMap)
	if err != nil {
		return fmt.Errorf("error constructing dashboard POST request: %v", err)
	}

	_, err = sendRequest(http.MethodPost, dashboardURL, requestData)
	if err != nil {
		return err
	}

	fmt.Println("Created dashboard with title", dashboardTitle)
	return nil
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	grafanaURL, err := getURL(scanner)
	if err != nil {
		fmt.Println("Error getting Grafana connection details: ", err)
		return
	}
	fmt.Println("\nStarting setup at ", grafanaURL)

	grafanaURL = grafanaURL.JoinPath("api")
	createURL := grafanaURL.JoinPath("serviceaccounts")

	// create new service account
	// append a random int to service account name to mitigate chance of account already existing
	fmt.Println("Creating service account...")
	serviceAccountName := "Grafana_Live_" + strconv.Itoa(rand.Int())
	id, err := createAdminServiceAccount(createURL, serviceAccountName)
	if err != nil {
		fmt.Println("Error creating service account:", err)
		return
	}

	// add token to service account
	fmt.Println("Generating service account token...")
	addTokenURL := grafanaURL.JoinPath("serviceaccounts", strconv.Itoa(id), "tokens")
	key, err := addServiceAccountToken(addTokenURL)
	if err != nil {
		fmt.Println("Error creating token:", err)
		return
	}

	// set .env file to contain token (or print token)
	fmt.Println("Adding service account token to .env file...")
	setEnvFile(scanner, key)

	// create the Grafana Live dashboard from JSON
	dashboardURL := grafanaURL.JoinPath("dashboards", "db")
	err = createDashboard(dashboardURL, DashboardJsonFile)
	if err != nil {
		fmt.Println("Error creating dashboard:", err)
		return
	}

	fmt.Println("Setup complete!")
}
