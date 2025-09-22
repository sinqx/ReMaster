package models

type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
