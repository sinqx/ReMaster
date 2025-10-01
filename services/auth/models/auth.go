package models

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserType string

const (
	UserTypeAnonymous UserType = "anonymous"
	UserTypeClient    UserType = "client"
	UserTypeMaster    UserType = "master"
	UserTypeAdmin     UserType = "admin"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string             `bson:"email" json:"email" validate:"required,email"`
	Password  string             `bson:"password" json:"-"`
	FirstName string             `bson:"first_name" json:"first_name" validate:"required,min=2,max=50"`
	LastName  string             `bson:"last_name" json:"last_name" validate:"required,min=2,max=50"`
	Phone     string             `bson:"phone" json:"phone" validate:"required"`
	UserType  UserType           `bson:"user_type" json:"user_type" validate:"required,oneof=client master admin"`

	GoogleID     string `bson:"google_id,omitempty" json:"google_id,omitempty"`
	GoogleEmail  string `bson:"google_email,omitempty" json:"google_email,omitempty"`
	ProfileImage string `bson:"profile_image,omitempty" json:"profile_image,omitempty"`

	IsActive   bool `bson:"is_active" json:"is_active"`
	IsVerified bool `bson:"is_verified" json:"is_verified"`

	CreatedAt       time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `bson:"updated_at" json:"updated_at"`
	LastLoginAt     *time.Time `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	EmailVerifiedAt *time.Time `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`

	LoginAttempts    int        `bson:"login_attempts" json:"login_attempts"`
	LastLoginIP      string     `bson:"last_login_ip,omitempty" json:"last_login_ip,omitempty"`
	PasswordChangeAt time.Time  `bson:"password_changed_at" json:"password_changed_at"`
	LockedUntil      *time.Time `bson:"locked_until,omitempty" json:"locked_until,omitempty"`
}

type UserResponse struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Phone        string     `json:"phone"`
	UserType     UserType   `json:"user_type"`
	ProfileImage string     `json:"profile_image,omitempty"`
	IsActive     bool       `json:"is_active"`
	IsVerified   bool       `json:"is_verified"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type RefreshToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token     string             `bson:"token" json:"token"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	IsRevoked bool               `bson:"is_revoked" json:"is_revoked"`
	DeviceID  string             `bson:"device_id,omitempty" json:"device_id,omitempty"`
	UserAgent string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	IP        string             `bson:"ip,omitempty" json:"ip,omitempty"`
}

type RefreshTokenResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	TokenType    string `json:"token_type"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type ValidateTokenRequest struct {
	AccessToken string
}

type ValidateTokenResponse struct {
	Valid      bool
	UserID     string
	UserType   UserType
	IsActive   bool
	IsVerified bool
	ExpiresAt  int64
}

type LoginAttempt struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string             `bson:"email" json:"email"`
	IP        string             `bson:"ip" json:"ip"`
	UserAgent string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	Success   bool               `bson:"success" json:"success"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	Reason    string             `bson:"reason,omitempty" json:"reason,omitempty"`
}

type RegisterRequest struct {
	Email     string   `json:"email" validate:"required,email"`
	Password  string   `json:"password" validate:"required,min=8"`
	FirstName string   `json:"first_name" validate:"required,min=2,max=50"`
	LastName  string   `json:"last_name" validate:"required,min=2,max=50"`
	Phone     string   `json:"phone" validate:"required,min=6,max=10"`
	UserType  UserType `json:"user_type" validate:"required,oneof=client master"`
}

type RegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	AuthResponse
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type OAuthLoginRequest struct {
	Provider string `json:"provider" validate:"required,oneof=google"`
	IDToken  string `json:"id_token" validate:"required"`
}

type ChangePasswordRequest struct {
	UserID      string `json:"user_id" validate:"required"`
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type AuthResponse struct {
	User         *UserResponse `json:"user"`
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresAt    int64         `json:"expires_at"`
	TokenType    string        `json:"token_type"`
}

type LogoutRequest struct {
	UserID       string `json:"user_id" validate:"required"`
	AccessToken  string `json:"access_token" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type RequestMetadata struct {
	UserAgent string
	IPAddress string
	DeviceID  string
}

// User information -> UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:           u.ID.Hex(),
		Email:        u.Email,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Phone:        u.Phone,
		UserType:     u.UserType,
		ProfileImage: u.ProfileImage,
		IsActive:     u.IsActive,
		IsVerified:   u.IsVerified,
		CreatedAt:    u.CreatedAt,
		LastLoginAt:  u.LastLoginAt,
	}
}

func (u *User) BeforeCreate() {
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	u.PasswordChangeAt = now
	u.IsActive = true
	u.IsVerified = false
	u.LoginAttempts = 0

	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
}

func (u *User) BeforeUpdate() {
	u.UpdatedAt = time.Now()
}

func (req *RegisterRequest) ValidateRegisterRequest() error {
	var errs []string

	if _, err := mail.ParseAddress(req.Email); err != nil {
		errs = append(errs, "invalid email format")
	}
	if len(req.Password) < 8 {
		errs = append(errs, "password must be at least 8 characters long")
	}
	if strings.TrimSpace(req.FirstName) == "" || len(req.FirstName) < 2 || len(req.FirstName) > 50 {
		errs = append(errs, "first name must be between 2 and 50 characters")
	}
	if strings.TrimSpace(req.LastName) == "" || len(req.LastName) < 2 || len(req.LastName) > 50 {
		errs = append(errs, "last name must be between 2 and 50 characters")
	}
	if req.UserType != UserTypeMaster && req.UserType != UserTypeClient {
		errs = append(errs, "invalid user type, must be 'client' or 'master'")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}
