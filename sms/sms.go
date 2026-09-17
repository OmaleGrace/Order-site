package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type sendOTPRequest struct {
	APIKey         string `json:"api_key"`
	MessageType    string `json:"message_type"`
	To             string `json:"to"`
	From           string `json:"from"`
	Channel        string `json:"channel"`
	PinAttempts    int    `json:"pin_attempts"`
	PinTimeToLive  int    `json:"pin_time_to_live"`
	PinLength      int    `json:"pin_length"`
	PinPlaceholder string `json:"pin_placeholder"`
	MessageText    string `json:"message_text"`
	PinType        string `json:"pin_type"`
}

type sendOTPResponse struct {
	PinID string `json:"pinId"`
}

func SendOTP(phone string) (string, error) {
	apiKey := os.Getenv("TERMII_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("TERMII_API_KEY not set")
	}

	reqBody := sendOTPRequest{
		APIKey:         apiKey,
		MessageType:    "NUMERIC",
		To:             phone,
		From:           "N-Alert",
		Channel:        "generic",
		PinAttempts:    3,
		PinTimeToLive:  10,
		PinLength:      6,
		PinPlaceholder: "< 123456 >",
		MessageText:    "Your Grace's Kitchen verification code is < 123456 >",
		PinType:        "NUMERIC",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(
		"https://api.ng.termii.com/api/sms/otp/send",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

fmt.Println("Termii status:", resp.Status)
fmt.Println("Termii send OTP raw response:", string(bodyBytes))

if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return "", fmt.Errorf(
        "termii returned HTTP %s: %s",
        resp.Status,
        string(bodyBytes),
    )
}

var result sendOTPResponse
if err := json.Unmarshal(bodyBytes, &result); err != nil {
    return "", err
}

if result.PinID == "" {
    return "", fmt.Errorf("termii did not return a pin ID")
}

	return result.PinID, nil
}

type verifyOTPRequest struct {
	APIKey string `json:"api_key"`
	PinID  string `json:"pin_id"`
	Pin    string `json:"pin"`
}

type verifyOTPResponse struct {
	Verified string `json:"verified"`
}

func VerifyOTP(pinID, code string) (bool, error) {
	apiKey := os.Getenv("TERMII_API_KEY")
	if apiKey == "" {
		return false, fmt.Errorf("TERMII_API_KEY not set")
	}

	reqBody := verifyOTPRequest{
		APIKey: apiKey,
		PinID:  pinID,
		Pin:    code,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return false, err
	}

	resp, err := http.Post(
		"https://api.ng.termii.com/api/sms/otp/verify",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Println("Termii verify OTP raw response:", string(bodyBytes))

	var result verifyOTPResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return false, err
	}

	return result.Verified == "True", nil
}