package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleOAuthService handles Google OAuth operations
type GoogleOAuthService struct {
	Config *oauth2.Config
}

// GoogleUserInfo represents Google user profile data
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

var Google *GoogleOAuthService

// InitGoogleOAuth initializes the Google OAuth service
func InitGoogleOAuth() error {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		// Google OAuth is optional - just log and continue
		fmt.Println("[GoogleOAuth] GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET not set, Google OAuth disabled")
		Google = nil
		return nil
	}

	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/api/v1/auth/google/callback"
	}

	Google = &GoogleOAuthService{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}

	fmt.Printf("[GoogleOAuth] Initialized with redirect URL: %s\n", redirectURL)
	return nil
}

// IsEnabled returns whether Google OAuth is enabled
func (g *GoogleOAuthService) IsEnabled() bool {
	return g != nil && g.Config != nil
}

// GetAuthURL generates the Google OAuth authorization URL
func (g *GoogleOAuthService) GetAuthURL(state string) string {
	return g.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges the authorization code for tokens
func (g *GoogleOAuthService) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetUserInfo retrieves user profile from Google using the access token
func (g *GoogleOAuthService) GetUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	client := g.Config.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google API error: %s", string(body))
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	if !userInfo.VerifiedEmail {
		return nil, errors.New("email not verified with Google")
	}

	return &userInfo, nil
}
