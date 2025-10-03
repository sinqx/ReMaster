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


// package oauth

// import (
// 	"context"
// 	"crypto/rand"
// 	"encoding/base64"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"time"

// 	"golang.org/x/oauth2"
// 	"golang.org/x/oauth2/facebook"
// 	"golang.org/x/oauth2/google"

// 	config "remaster/shared"

// 	"cloud.google.com/go/auth/credentials/idtoken"
// )

// type ProviderType string

// const (
// 	Google   ProviderType = "google"
// 	Facebook ProviderType = "facebook"
// )

// type UserInfo struct {
// 	ProviderID    string 
// 	Email         string
// 	EmailVerified bool
// 	FirstName     string
// 	LastName      string
// 	Picture       string
// 	Provider      ProviderType
// }

// type OAuthProvider interface {
// 	GetAuthURL(state string) string

// 	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)

// 	GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)
// }

// // ============================================================================
// // GOOGLE PROVIDER
// // ============================================================================

// type GoogleProvider struct {
// 	config *oauth2.Config
// }

// func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
// 	return &GoogleProvider{
// 		config: &oauth2.Config{
// 			ClientID:     clientID,
// 			ClientSecret: clientSecret,
// 			RedirectURL:  redirectURL,
// 			Scopes: []string{
// 				"https://www.googleapis.com/auth/userinfo.email",
// 				"https://www.googleapis.com/auth/userinfo.profile",
// 				"openid",
// 			},
// 			Endpoint: google.Endpoint,
// 		},
// 	}
// }

// func (p *GoogleProvider) GetAuthURL(state string) string {
// 	return p.config.AuthCodeURL(state,
// 		oauth2.AccessTypeOffline, 
// 		oauth2.ApprovalForce,     
// 	)
// }

// func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
// 	token, err := p.config.Exchange(ctx, code)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to exchange code: %w", err)
// 	}
// 	return token, nil
// }

// func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
// 	idToken, ok := token.Extra("id_token").(string)
// 	if !ok {
// 		return nil, fmt.Errorf("no id_token in response")
// 	}

// 	payload, err := idtoken.Validate(ctx, idToken, p.config.ClientID)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid id_token: %w", err)
// 	}

// 	email, _ := payload.Claims["email"].(string)
// 	emailVerified, _ := payload.Claims["email_verified"].(bool)
// 	givenName, _ := payload.Claims["given_name"].(string)
// 	familyName, _ := payload.Claims["family_name"].(string)
// 	picture, _ := payload.Claims["picture"].(string)
// 	sub, _ := payload.Claims["sub"].(string) // unique Google ID

// 	return &UserInfo{
// 		ProviderID:    sub,
// 		Email:         email,
// 		EmailVerified: emailVerified,
// 		FirstName:     givenName,
// 		LastName:      familyName,
// 		Picture:       picture,
// 		Provider:      Google,
// 	}, nil
// }

// // ============================================================================
// // FACEBOOK PROVIDER
// // ============================================================================

// type FacebookProvider struct {
// 	config *oauth2.Config
// }

// func NewFacebookProvider(appID, appSecret, redirectURL string) *FacebookProvider {
// 	return &FacebookProvider{
// 		config: &oauth2.Config{
// 			ClientID:     appID,
// 			ClientSecret: appSecret,
// 			RedirectURL:  redirectURL,
// 			Scopes:       []string{"email", "public_profile"},
// 			Endpoint:     facebook.Endpoint,
// 		},
// 	}
// }

// func (p *FacebookProvider) GetAuthURL(state string) string {
// 	return p.config.AuthCodeURL(state)
// }

// func (p *FacebookProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
// 	token, err := p.config.Exchange(ctx, code)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to exchange code: %w", err)
// 	}
// 	return token, nil
// }

// func (p *FacebookProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
// 	client := p.config.Client(ctx, token)

// 	resp, err := client.Get("https://graph.facebook.com/me?fields=id,email,first_name,last_name,picture")
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get user info: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("facebook API returned status %d", resp.StatusCode)
// 	}

// 	var fbUser struct {
// 		ID        string `json:"id"`
// 		Email     string `json:"email"`
// 		FirstName string `json:"first_name"`
// 		LastName  string `json:"last_name"`
// 		Picture   struct {
// 			Data struct {
// 				URL string `json:"url"`
// 			} `json:"data"`
// 		} `json:"picture"`
// 	}

// 	if err := json.NewDecoder(resp.Body).Decode(&fbUser); err != nil {
// 		return nil, fmt.Errorf("failed to decode response: %w", err)
// 	}

// 	return &UserInfo{
// 		ProviderID:    fbUser.ID,
// 		Email:         fbUser.Email,
// 		EmailVerified: true, // Facebook emails are verified
// 		FirstName:     fbUser.FirstName,
// 		LastName:      fbUser.LastName,
// 		Picture:       fbUser.Picture.Data.URL,
// 		Provider:      Facebook,
// 	}, nil
// }

// // ============================================================================
// // PROVIDER FACTORY
// // ============================================================================

// type ProviderFactory struct {
// 	providers  map[ProviderType]OAuthProvider
// 	stateStore StateStore 
// }

// func NewProviderFactory(cfg *config.OAuthConfig, stateStore StateStore) *ProviderFactory {
// 	return &ProviderFactory{
// 		providers: map[ProviderType]OAuthProvider{
// 			Google: NewGoogleProvider(
// 				cfg.GoogleClientID,
// 				cfg.GoogleClientSecret,
// 				cfg.GoogleRedirectURL,
// 			),
// 			Facebook: NewFacebookProvider(
// 				cfg.FacebookAppID,
// 				cfg.FacebookAppSecret,
// 				cfg.FacebookRedirectURL,
// 			),
// 		},
// 		stateStore: stateStore,
// 	}
// }

// func (f *ProviderFactory) GetProvider(provider ProviderType) (OAuthProvider, error) {
// 	p, ok := f.providers[provider]
// 	if !ok {
// 		return nil, fmt.Errorf("provider %s not supported", provider)
// 	}
// 	return p, nil
// }

// func (f *ProviderFactory) GenerateStateToken(ctx context.Context) (string, error) {
// 	b := make([]byte, 32)
// 	if _, err := rand.Read(b); err != nil {
// 		return "", err
// 	}

// 	state := base64.URLEncoding.EncodeToString(b)

// 	if err := f.stateStore.SaveState(ctx, state, 10*time.Minute); err != nil {
// 		return "", err
// 	}

// 	return state, nil
// }

// // ValidateStateToken проверяет state token
// func (f *ProviderFactory) ValidateStateToken(ctx context.Context, state string) error {
// 	valid, err := f.stateStore.ValidateState(ctx, state)
// 	if err != nil {
// 		return err
// 	}
// 	if !valid {
// 		return fmt.Errorf("invalid state token")
// 	}
// 	return nil
// }

// ============================================================================
// STATE STORE (Redis)
// ============================================================================

// type StateStore interface {
// 	SaveState(ctx context.Context, state string, ttl time.Duration) error
// 	ValidateState(ctx context.Context, state string) (bool, error)
// }

// type RedisStateStore struct {
// 	redis *redis.Client
// }

// func NewRedisStateStore(redis *redis.Client) *RedisStateStore {
// 	return &RedisStateStore{redis: redis}
// }

// func (s *RedisStateStore) SaveState(ctx context.Context, state string, ttl time.Duration) error {
// 	key := fmt.Sprintf("oauth:state:%s", state)
// 	return s.redis.Set(ctx, key, "1", ttl).Err()
// }

// func (s *RedisStateStore) ValidateState(ctx context.Context, state string) (bool, error) {
// 	key := fmt.Sprintf("oauth:state:%s", state)

// 	result, err := s.redis.GetDel(ctx, key).Result()
// 	if err == redis.Nil {
// 		return false, nil
// 	}
// 	if err != nil {
// 		return false, err
// 	}

// 	return result == "1", nil
// }
