package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"rumble/client/internal/netcfg"
)

// getAuthToken retrieves the current user token for authorization
func getAuthToken() string {
	// TODO: Implement proper token retrieval
	// For now, stub with empty token
	return ""
}

// getBaseURL returns the API base URL
func getBaseURL() string {
	return netcfg.APIBase
}

// GetJSON performs a GET request and decodes the JSON response
func GetJSON[T any](path string) (T, error) {
	var result T

	url := getBaseURL() + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, err
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

// PostJSON performs a POST request with JSON body and decodes the JSON response
func PostJSON[Req any, Res any](body Req, path string) (Res, error) {
	var result Res

	jsonData, err := json.Marshal(body)
	if err != nil {
		return result, err
	}

	url := getBaseURL() + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	err = json.Unmarshal(bodyBytes, &result)
	return result, err
}
