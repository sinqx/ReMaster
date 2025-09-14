package models

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Success bool   `json:"success"`
}

type SuccessResponse struct {
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Success bool   `json:"success"`
}
