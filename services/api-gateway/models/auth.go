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

type LoginDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OAuthTokenRequest struct {
	Provider string `json:"provider" binding:"required,oneof=google facebook"`
	IDToken  string `json:"id_token" binding:"required"`
}
