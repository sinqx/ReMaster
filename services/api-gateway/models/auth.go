package models

type RegisterDTO struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	UserType  string `json:"user_type"`
}

type AuthResponse struct {
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	UserType     string `json:"user_type"`
}

type RefreshTokenResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type ValidateTokenResponse struct {
	Valid     bool   `json:"valid"`
	UserID    string `json:"user_id"`
	UserType  string `json:"user_type"`
	ExpiresAt int64  `json:"expires_at"`
}

type LoginDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OAuthTokenRequest struct {
	Provider string `json:"provider" binding:"required,oneof=google facebook"`
	IDToken  string `json:"id_token" binding:"required"`
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp int64             `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}
