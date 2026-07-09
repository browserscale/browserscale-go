package browserscale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type rentRequest struct {
	RentDuration             int      `json:"rentDuration"`
	APIKey                   string   `json:"apiKey"`
	ProxyHost                string   `json:"proxyHost"`
	ProxyPort                int      `json:"proxyPort"`
	ProxyUsername            string   `json:"proxyUsername"`
	ProxyPassword            string   `json:"proxyPassword"`
	CountryCode              string   `json:"countryCode,omitempty"`
	Timezone                 string   `json:"timezone,omitempty"`
	Fingerprint              string   `json:"fingerprint,omitempty"`
	WebGLRenderer            string   `json:"webglRenderer,omitempty"`
	WebGLVendor              string   `json:"webglVendor,omitempty"`
	WebGLSupportedExtensions []string `json:"webglSupportedExtensions,omitempty"`
}

type rentResponse struct {
	Success        bool   `json:"success"`
	Error          string `json:"error"`
	GrpcUrl        string `json:"grpcUrl"`
	SessionId      string `json:"sessionId"`
	CountryCode    string `json:"countryCode,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	AcceptLanguage string `json:"acceptLanguage,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
}

type stopRequest struct {
	SessionId string `json:"sessionId"`
	APIKey    string `json:"apiKey"`
}

type stopResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

var ApiEndpoint = "https://api.browserscale.cloud"

func setApiEndpoint(endpoint string) {
	ApiEndpoint = endpoint
}

func callRentApi(config *BrowserConfig) (*rentResponse, error) {
	rentData := rentRequest{
		RentDuration:  config.rentDuration,
		APIKey:        config.apiKey,
		ProxyHost:     config.proxyHost,
		ProxyPort:     config.proxyPort,
		ProxyUsername: config.proxyUsername,
		ProxyPassword: config.proxyPassword,
	}
	if config.countryCode != "" {
		rentData.CountryCode = config.countryCode
	}
	if config.timezone != "" {
		rentData.Timezone = config.timezone
	}
	if config.fingerprint != "" {
		rentData.Fingerprint = config.fingerprint
	}
	if config.webglRenderer != "" {
		rentData.WebGLRenderer = config.webglRenderer
	}
	if config.webglVendor != "" {
		rentData.WebGLVendor = config.webglVendor
	}
	if len(config.webglExtensions) > 0 {
		rentData.WebGLSupportedExtensions = config.webglExtensions
	}

	rentJSON, err := json.Marshal(rentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rent request: %v", err)
	}

	resp, err := http.Post(ApiEndpoint+"/rent", "application/json", bytes.NewBuffer(rentJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to rent browser: %v", err)
	}
	defer resp.Body.Close()

	var rentResp rentResponse
	if err := json.NewDecoder(resp.Body).Decode(&rentResp); err != nil {
		return nil, fmt.Errorf("failed to decode rent response: %v", err)
	}
	if !rentResp.Success {
		return nil, fmt.Errorf("rent session failed: %s", rentResp.Error)
	}
	return &rentResp, nil
}

func callStopBrowserApi(apiKey string, sessionId string) error {
	stopData := stopRequest{SessionId: sessionId, APIKey: apiKey}
	stopJSON, err := json.Marshal(stopData)
	if err != nil {
		return fmt.Errorf("failed to marshal stop request: %v", err)
	}

	resp, err := http.Post(ApiEndpoint+"/stop", "application/json", bytes.NewBuffer(stopJSON))
	if err != nil {
		return fmt.Errorf("failed to stop browser: %v", err)
	}
	defer resp.Body.Close()

	var response stopResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode stop response: %v", err)
	}
	if !response.Success {
		return fmt.Errorf("failed to stop browser: %s", response.Error)
	}
	return nil
}
