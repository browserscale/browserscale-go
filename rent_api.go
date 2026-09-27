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
	GpuEnabled               bool     `json:"gpuEnabled,omitempty"`
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
	if config.gpuEnabled {
		rentData.GpuEnabled = true
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

type stopAllResponse struct {
	Success bool   `json:"success"`
	Stopped int    `json:"stopped"`
	Error   string `json:"error,omitempty"`
}

func callStopAllApi(apiKey string) (int, error) {
	stopJSON, err := json.Marshal(listSessionsRequest{APIKey: apiKey})
	if err != nil {
		return 0, fmt.Errorf("failed to marshal stopAll request: %v", err)
	}

	resp, err := http.Post(ApiEndpoint+"/stopAll", "application/json", bytes.NewBuffer(stopJSON))
	if err != nil {
		return 0, fmt.Errorf("failed to stop browsers: %v", err)
	}
	defer resp.Body.Close()

	var response stopAllResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("failed to decode stopAll response: %v", err)
	}
	if !response.Success {
		return 0, fmt.Errorf("failed to stop browsers: %s", response.Error)
	}
	return response.Stopped, nil
}

type listSessionsRequest struct {
	APIKey string `json:"apiKey"`
}

type listSessionsResponse struct {
	Success  bool          `json:"success"`
	Sessions []BrowserInfo `json:"sessions"`
	Error    string        `json:"error,omitempty"`
}

func callListSessionsApi(apiKey string) ([]BrowserInfo, error) {
	listJSON, err := json.Marshal(listSessionsRequest{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list request: %v", err)
	}

	resp, err := http.Post(ApiEndpoint+"/sessions", "application/json", bytes.NewBuffer(listJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to list browsers: %v", err)
	}
	defer resp.Body.Close()

	// Named explicitly, because this is the one endpoint a caller can reach on a
	// deployment that does not have it: listing came after rent and stop. Letting
	// it fall through would report a JSON decode failure against an error page,
	// which says nothing about the actual problem.
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("the API at %s does not support listing sessions", ApiEndpoint)
	}

	var response listSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %v", err)
	}
	if !response.Success {
		return nil, fmt.Errorf("failed to list browsers: %s", response.Error)
	}
	return response.Sessions, nil
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
