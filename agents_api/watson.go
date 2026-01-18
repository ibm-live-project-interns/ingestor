package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

/* ---------------- IAM TOKEN CACHE ---------------- */

var (
	iamToken    string
	tokenExpiry time.Time
	tokenMutex  sync.Mutex
)

/* ---------------- GET IAM TOKEN ---------------- */

func getIAMToken(apiKey string) (string, error) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	if iamToken != "" && time.Now().Before(tokenExpiry) {
		return iamToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	data.Set("apikey", apiKey)

	req, err := http.NewRequest(
		"POST",
		"https://iam.cloud.ibm.com/identity/token",
		bytes.NewBufferString(data.Encode()),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("IAM auth failed %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	iamToken = tokenResp.AccessToken
	tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return iamToken, nil
}

/* ---------------- JSON EXTRACTOR ---------------- */

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return text[start : end+1]
}

/* ---------------- CALL WATSONX ---------------- */

func CallWatsonAI(event Event) (UnifiedResponse, error) {
	apiKey := os.Getenv("WATSONX_API_KEY")
	region := os.Getenv("WATSONX_REGION")
	projectID := os.Getenv("WATSONX_PROJECT_ID")

	if apiKey == "" || region == "" || projectID == "" {
		return UnifiedResponse{}, errors.New("Watsonx env vars missing")
	}

	token, err := getIAMToken(apiKey)
	if err != nil {
		return UnifiedResponse{}, err
	}

	endpoint := fmt.Sprintf(
		"https://%s.ml.cloud.ibm.com/ml/v1/text/generation?version=2024-01-10",
		region,
	)

	// ✅ REQUIRED PROMPT FORMAT (as per spec)
	prompt := fmt.Sprintf(
		`<System data>
Event type: %s
Event message: %s
</System data>

<Instructions>
Use the system data to answer the question.
Do NOT mention system data or how you derived the answer.
Respond ONLY in valid JSON with fields:
severity, explanation, recommended_action.
</Instructions>

<Question>
What is the severity of the event and what action should be taken?
</Question>`,
		event.Type,
		event.Message,
	)

	payload := map[string]interface{}{
		"model_id":   "ibm/granite-3-8b-instruct",
		"project_id": projectID,
		"input":      prompt,
		"parameters": map[string]interface{}{
			"temperature":    0.2,
			"max_new_tokens": 200,
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return UnifiedResponse{}, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return UnifiedResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return UnifiedResponse{}, fmt.Errorf("Watsonx failed %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Results []struct {
			GeneratedText string `json:"generated_text"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return UnifiedResponse{}, err
	}

	if len(res.Results) == 0 {
		return UnifiedResponse{}, errors.New("empty response from Watsonx")
	}

	cleanJSON := extractJSON(res.Results[0].GeneratedText)
	if cleanJSON == "" {
		return UnifiedResponse{
			Severity:          "unknown",
			Explanation:       res.Results[0].GeneratedText,
			RecommendedAction: "Manual review required",
		}, nil
	}

	var ai UnifiedResponse
	if err := json.Unmarshal([]byte(cleanJSON), &ai); err != nil {
		return UnifiedResponse{
			Severity:          "unknown",
			Explanation:       cleanJSON,
			RecommendedAction: "Manual review required",
		}, nil
	}

	return ai, nil
}
