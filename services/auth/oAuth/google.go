package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	config "remaster/shared"
)

// GoogleUserInfo - structure for user data from Google.
type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// GoogleAuthClient encapsulates all the logic for working with Google OAuth.
type GoogleAuthClient struct {
	config *oauth2.Config
}

// NewGoogleAuthClient - constructor for our client.
func NewGoogleAuthClient(oauthCfg *config.OAuthConfig) *GoogleAuthClient {
	return &GoogleAuthClient{
		config: &oauth2.Config{
			ClientID:     oauthCfg.GoogleClientID,
			ClientSecret: oauthCfg.GoogleClientSecret,
			RedirectURL:  oauthCfg.GoogleRedirectURL,
			Scopes:       []string{"email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// GetAuthCodeURL returns the link for redirection.
func (g *GoogleAuthClient) GetAuthCodeURL(state string) string {
	return g.config.AuthCodeURL(state)
}

// ExchangeCode exchanges the code for user information.
func (g *GoogleAuthClient) ExchangeCode(ctx context.Context, code string) (*GoogleUserInfo, error) {
	// Exchange the code for a Google token
	googleToken, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Request user information
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + googleToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info body: %w", err)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user info: %w", err)
	}

	return &userInfo, nil
}