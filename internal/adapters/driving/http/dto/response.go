package dto

// StandardResponse is the standard API response envelope
type StandardResponse struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    any `json:"data,omitempty"`
}

// ErrorResponse is the error API response envelope
type ErrorResponse struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    any `json:"data,omitempty"`
}

// SuccessResponse creates a success response struct
func SuccessResponse(data any) StandardResponse {
	return StandardResponse{
		Message: "Success",
		Success: true,
		Data:    data,
	}
}

// FailResponse creates a failure response struct
func FailResponse(message string) ErrorResponse {
	return ErrorResponse{
		Message: message,
		Success: false,
		Data:    nil,
	}
}
