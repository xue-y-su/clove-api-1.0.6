package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	statusEndpoint     = "http://127.0.0.1:3000/api/status"
	maxStatusBodyBytes = 1 << 20
)

type statusPayload struct {
	Success bool `json:"success"`
}

func checkStatus(client *http.Client) error {
	response, err := client.Get(statusEndpoint)
	if err != nil {
		return fmt.Errorf("request status endpoint: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("status endpoint returned HTTP %d", response.StatusCode)
	}

	var status statusPayload
	if err := json.NewDecoder(io.LimitReader(response.Body, maxStatusBodyBytes)).Decode(&status); err != nil {
		return fmt.Errorf("decode status response: %w", err)
	}
	if !status.Success {
		return fmt.Errorf("status endpoint reported failure")
	}
	return nil
}

func main() {
	client := &http.Client{Timeout: 5 * time.Second}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if err := checkStatus(client); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
