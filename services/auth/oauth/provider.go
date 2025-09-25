package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	config "remaster/shared"

	"cloud.google.com/go/auth/credentials/idtoken"
)

type ProviderType string

const (
	Google   ProviderType = "google"
	Facebook ProviderType = "facebook"
)

type Claims struct {
	Email     string
	FirstName string
	LastName  string
	Picture   string
}

type OAuthProvider interface {
	VerifyIDToken(ctx context.Context, token string) (*Claims, error)
}

// ==== Google ====

type GoogleProvider struct {
	clientID string
}

func NewGoogleProvider(clientID string) *GoogleProvider {
	return &GoogleProvider{clientID: clientID}
}

func (p *GoogleProvider) VerifyIDToken(ctx context.Context, idToken string) (*Claims, error) {
	payload, err := idtoken.Validate(ctx, idToken, p.clientID)
	if err != nil {
		return nil, fmt.Errorf("google: token validation failed: %w", err)
	}

	email, _ := payload.Claims["email"].(string)
	fullName, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	parts := strings.Fields(fullName)
	firstName := fullName
	lastName := ""
	if len(parts) > 1 {
		firstName = parts[0]
		lastName = strings.Join(parts[1:], " ")
	}

	return &Claims{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Picture:   picture,
	}, nil
}

// ==== Facebook ====
// NOTE: not tested

type FacebookProvider struct {
	appID     string
	appSecret string
}

func NewFacebookProvider(appID, appSecret string) *FacebookProvider {
	return &FacebookProvider{appID: appID, appSecret: appSecret}
}

func (p *FacebookProvider) VerifyIDToken(ctx context.Context, accessToken string) (*Claims, error) {
	debugURL := fmt.Sprintf(
		"https://graph.facebook.com/debug_token?input_token=%s&access_token=%s|%s",
		accessToken, p.appID, p.appSecret,
	)

	resp, err := http.Get(debugURL)
	if err != nil {
		return nil, fmt.Errorf("facebook: debug request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook: debug request failed with status %d", resp.StatusCode)
	}

	var debugResp struct {
		Data struct {
			IsValid bool `json:"is_valid"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&debugResp); err != nil {
		return nil, fmt.Errorf("facebook: failed to decode debug response: %w", err)
	}

	if !debugResp.Data.IsValid {
		return nil, fmt.Errorf("facebook: token is invalid")
	}

	userURL := fmt.Sprintf(
		"https://graph.facebook.com/me?fields=id,first_name,last_name,email,picture&access_token=%s",
		accessToken,
	)

	resp, err = http.Get(userURL)
	if err != nil {
		return nil, fmt.Errorf("facebook: user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facebook: user request failed with status %d", resp.StatusCode)
	}

	var userResp struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Picture   struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("facebook: failed to decode user response: %w", err)
	}

	return &Claims{
		Email:     userResp.Email,
		FirstName: userResp.FirstName,
		LastName:  userResp.LastName,
		Picture:   userResp.Picture.Data.URL,
	}, nil
}

// ==== Factory ====

type ProviderFactory struct {
	providers map[ProviderType]OAuthProvider
}

func NewProviderFactory(cfg *config.OAuthConfig) *ProviderFactory {
	return &ProviderFactory{
		providers: map[ProviderType]OAuthProvider{
			Google:   NewGoogleProvider(cfg.GoogleClientID),
			Facebook: NewFacebookProvider(cfg.FacebookAppID, cfg.FacebookAppSecret), // not tested
		},
	}
}

func (f *ProviderFactory) GetProvider(provider ProviderType) (OAuthProvider, error) {
	if p, ok := f.providers[provider]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("provider %s not supported", provider)
}
