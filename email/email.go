package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type recipient struct {
	Email string `json:"email"`
}

type sender struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type emailRequest struct {
	Sender      sender      `json:"sender"`
	To          []recipient `json:"to"`
	Subject     string      `json:"subject"`
	HTMLContent string      `json:"htmlContent"`
}

func Send(toEmail, subject, htmlContent string) error {
	apiKey := os.Getenv("BREVO_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("BREVO_API_KEY not set")
	}

	reqBody := emailRequest{
		Sender: sender{
			Name:  "Grace's Kitchen",
			Email: "omalegrace2009@gmail.com",
		},
		To: []recipient{
			{Email: toEmail},
		},
		Subject:     subject,
		HTMLContent: htmlContent,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("brevo API returned status %d", resp.StatusCode)
	}

	return nil
}