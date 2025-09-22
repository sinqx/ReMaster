package models



type RegisterResponse struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

type RegisterDTO struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	UserType  string `json:"user_type"`
}
