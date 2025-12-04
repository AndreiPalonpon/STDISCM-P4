package util

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JSONResponse structure for successful responses
type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// JSONError structure for error responses
type JSONError struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// WriteJSON is a helper to write JSON responses
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Create the final response object structure based on payload type
	var response interface{}

	// If payload is already a map with a "success" key, use it directly (custom format)
	if responseMap, ok := payload.(map[string]interface{}); ok && responseMap["success"] != nil {
		response = payload
	} else if status >= 200 && status < 300 {
		// Standard success response wrapper
		response = JSONResponse{Success: true, Data: payload}
	} else {
		// Fallback for errors if WriteJSONError wasn't used
		response = JSONError{Success: false, Message: "Unknown error"}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}

// WriteJSONError is a helper to write standardized error JSON responses
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	log.Printf("HTTP Error %d: %s", status, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errorResponse := JSONError{
		Success: false,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Error writing JSON error response: %v", err)
	}
}

// HandleGRPCError translates gRPC status errors to appropriate HTTP responses.
// This is the core fault tolerance error mapping logic for the Gateway.
// It relies on WriteJSONError being defined in the same package.
func HandleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, treat as internal server error
		WriteJSONError(w, http.StatusInternalServerError, "Internal server error: Non-gRPC error occurred")
		return
	}

	// Map gRPC codes to HTTP status codes
	switch st.Code() {
	case codes.InvalidArgument:
		WriteJSONError(w, http.StatusBadRequest, st.Message())
	case codes.Unauthenticated:
		WriteJSONError(w, http.StatusUnauthorized, st.Message())
	case codes.PermissionDenied:
		WriteJSONError(w, http.StatusForbidden, st.Message())
	case codes.NotFound:
		WriteJSONError(w, http.StatusNotFound, st.Message())
	case codes.AlreadyExists:
		WriteJSONError(w, http.StatusConflict, st.Message())
	case codes.Unavailable:
		// Important for distributed systems: Service is down or unreachable
		WriteJSONError(w, http.StatusServiceUnavailable, "Service Unavailable: The backend service is unreachable.")
	case codes.DeadlineExceeded:
		WriteJSONError(w, http.StatusGatewayTimeout, "Service Timeout: The backend service took too long to respond.")
	default:
		// Catch-all for internal or unknown gRPC errors
		WriteJSONError(w, http.StatusInternalServerError, st.Message())
	}
}

// ExtractToken extracts the token from the Authorization header (Bearer <token>)
func ExtractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	// Expect header: "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}
