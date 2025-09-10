package handlers

import (
	"log/slog"

	auth_pb "remaster/shared/proto/auth"

	"remaster/services/auth/services"
)

type AuthHandler struct {
	auth_pb.UnimplementedAuthServiceServer
	authService *services.AuthService
	logger      *slog.Logger
}

func NewAuthHandler(authService *services.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// func (c *AuthHandler) Register(ctx context.Context, req *models.RegisterRequest) (*models.RegisterResponse, error) {
// 	// Logger.Info("Registration request for email: %s", req.Email)

// 	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
// 		return &models.RegisterResponse{
// 			Success: false,
// 			Message: "All fields are required",
// 		}, status.Error(codes.InvalidArgument, "missing required fields")
// 	}

// 	userType := models.UserTypeClient
// 	switch req.UserType {
// 	case models.UserType_USER_TYPE_CLIENT:
// 		userType = models.UserTypeClient
// 	case models.UserType_USER_TYPE_MASTER:
// 		userType = models.UserTypeMaster
// 	case models.UserType_USER_TYPE_ADMIN:
// 		userType = models.UserTypeAdmin
// 	default:
// 		userType = models.UserTypeClient
// 	}

// 	registerReq := &models.CreateUserRequest{
// 		Email:     req.Email,
// 		Password:  req.Password,
// 		FirstName: req.FirstName,
// 		LastName:  req.LastName,
// 		Phone:     req.Phone,
// 		UserType:  userType,
// 	}

// 	authResp, err := c.authService.Register(ctx, registerReq)
// 	if err != nil {
// 		// logger.Printf("Registration failed for %s: %v", req.Email, err)

// 		// if services.IsValidationError(err) {
// 		// 	return &models.RegisterResponse{
// 		// 		Success: false,
// 		// 		Message: err.Error(),
// 		// 	}, status.Error(codes.InvalidArgument, err.Error())
// 		// }

// 		// if services.IsConflictError(err) {
// 		// 	return &models.RegisterResponse{
// 		// 		Success: false,
// 		// 		Message: "User already exists",
// 		// 	}, status.Error(codes.AlreadyExists, err.Error())
// 		// }

// 		return &models.RegisterResponse{
// 			Success: false,
// 			Message: "Registration failed",
// 		}, status.Error(codes.Internal, "internal server error")
// 	}

// 	// logger.Printf("User registered successfully: %s", authResp.User.ID)

// 	return &models.RegisterResponse{
// 		Success:      true,
// 		Message:      "Registration successful",
// 		UserId:       authResp.User.ID,
// 		AccessToken:  authResp.AccessToken,
// 		RefreshToken: authResp.RefreshToken,
// 		ExpiresAt:    authResp.ExpiresAt,
// 	}, nil
// }
