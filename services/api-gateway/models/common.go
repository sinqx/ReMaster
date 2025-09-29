package models

type BaseResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

type SuccessResponse struct {
	BaseResponse
	Data any `json:"data,omitempty"`
}

type ErrorResponse struct {
	BaseResponse
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}
