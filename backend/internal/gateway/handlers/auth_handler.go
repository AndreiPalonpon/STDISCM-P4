package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"stdiscm_p4/backend/internal/gateway/util" // Assuming a utility package for JSON response handling
	pb "stdiscm_p4/backend/internal/pb/auth"   // Assuming the gRPC generated package
)

// AuthHandler holds the gRPC client for the Auth Service.
type AuthHandler struct {
	AuthClient pb.AuthServiceClient
}

// RESTLoginRequest mirrors the expected JSON input for /auth/login
type RESTLoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// RESTChangePasswordRequest mirrors the expected JSON input for /auth/change-password
type RESTChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// Helper function to extract token from Authorization header (Bearer <token>)
func extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

// handleGRPCError translates gRPC status errors to appropriate HTTP responses.
func handleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, treat as internal server error
		util.WriteJSONError(w, http.StatusInternalServerError, "Internal server error: Non-gRPC error occurred")
		return
	}

	// Map gRPC codes to HTTP status codes
	switch st.Code() {
	case codes.InvalidArgument:
		util.WriteJSONError(w, http.StatusBadRequest, st.Message())
	case codes.Unauthenticated:
		util.WriteJSONError(w, http.StatusUnauthorized, st.Message())
	case codes.PermissionDenied:
		util.WriteJSONError(w, http.StatusForbidden, st.Message())
	case codes.NotFound:
		util.WriteJSONError(w, http.StatusNotFound, st.Message())
	case codes.Unavailable:
		// Important for distributed systems: Service is down or unreachable
		util.WriteJSONError(w, http.StatusServiceUnavailable, fmt.Sprintf("Service Unavailable: %s", st.Message()))
	default:
		// Catch-all for internal or unknown gRPC errors
		util.WriteJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Backend error: %s", st.Message()))
	}
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var reqBody RESTLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		if errors.Is(err, io.EOF) {
			util.WriteJSONError(w, http.StatusBadRequest, "Request body is empty")
			return
		}
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Input validation (basic check before sending to gRPC)
	if reqBody.Identifier == "" || reqBody.Password == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Identifier and password are required")
		return
	}

	// Prepare gRPC request
	grpcReq := &pb.LoginRequest{
		Identifier: reqBody.Identifier,
		Password:   reqBody.Password,
	}

	// Use a context with a timeout for the gRPC call (e.g., 10 seconds)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Call the backend service
	grpcResp, err := h.AuthClient.Login(ctx, grpcReq)
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	// Check for a non-success response, which might contain a user-friendly message
	if !grpcResp.Success {
		// The service already returned a successful gRPC call but the login logic failed
		util.WriteJSONError(w, http.StatusUnauthorized, grpcResp.Message)
		return
	}

	// Map gRPC response to HTTP response format
	response := map[string]interface{}{
		"success": true,
		"token":   grpcResp.Token,
		"user":    grpcResp.User, // Protobuf fields convert cleanly to JSON
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Logout requires extracting the token from the header
	token, err := extractToken(r)
	if err != nil {
		// If token is missing or invalid format, we can still treat it as a successful "logout"
		// or return unauthorized if we want stricter adherence, but for logout, successful removal
		// of an unknown token is fine (idempotent). We'll return 200/OK if token extraction fails.
		util.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Logged out successfully (session token not provided or invalid format)",
		})
		return
	}

	// Prepare gRPC request
	grpcReq := &pb.LogoutRequest{
		Token: token,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Call the backend service
	grpcResp, err := h.AuthClient.Logout(ctx, grpcReq)
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	// Map gRPC response to HTTP response format
	// Logout is generally successful from the client perspective regardless of backend status
	response := map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// ValidateToken handles GET /auth/validate
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	token, err := extractToken(r)
	if err != nil {
		// If token is missing, fail validation immediately
		util.WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid":   false,
			"message": "Authorization token missing or invalid format",
		})
		return
	}

	// Prepare gRPC request
	grpcReq := &pb.ValidateTokenRequest{
		Token: token,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Call the backend service
	grpcResp, err := h.AuthClient.ValidateToken(ctx, grpcReq)
	if err != nil {
		// If gRPC call fails (e.g., service unavailable), return 503/500
		handleGRPCError(w, err)
		return
	}

	// If the token is invalid (checked by the backend logic), return 401
	if !grpcResp.Valid {
		util.WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid":   false,
			"message": grpcResp.Message,
		})
		return
	}

	// Map gRPC response to HTTP response format
	response := map[string]interface{}{
		"valid":   true,
		"user":    grpcResp.User,
		"message": "Token is valid",
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// ChangePassword handles POST /auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication: Need to validate the user's token and get their ID
	validateResp, authErr := h.authenticateRequest(r)
	if authErr != nil {
		// authenticateRequest handles writing the error response
		return
	}

	userID := validateResp.User.Id

	// 2. Decode Request Body
	var reqBody RESTChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		if errors.Is(err, io.EOF) {
			util.WriteJSONError(w, http.StatusBadRequest, "Request body is empty")
			return
		}
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// 3. Input Validation
	if reqBody.OldPassword == "" || reqBody.NewPassword == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Old and new passwords are required")
		return
	}
	if reqBody.OldPassword == reqBody.NewPassword {
		util.WriteJSONError(w, http.StatusBadRequest, "New password cannot be the same as the old password")
		return
	}

	// 4. Prepare and Call gRPC
	grpcReq := &pb.ChangePasswordRequest{
		UserId:      userID,
		OldPassword: reqBody.OldPassword,
		NewPassword: reqBody.NewPassword,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcResp, err := h.AuthClient.ChangePassword(ctx, grpcReq)
	if err != nil {
		handleGRPCError(w, err)
		return
	}

	// 5. Handle Business Logic Failure
	if !grpcResp.Success {
		// The service explicitly returned failure (e.g., "incorrect old password")
		util.WriteJSONError(w, http.StatusForbidden, grpcResp.Message)
		return
	}

	// 6. Success Response
	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": grpcResp.Message,
	})
}

// authenticateRequest is a gateway-level helper to validate the token in the request.
// This implements step 2 of the communication flow: "Gateway → AuthService.ValidateToken(token) → verify JWT"
func (h *AuthHandler) authenticateRequest(r *http.Request) (*pb.ValidateTokenResponse, error) {
	w := r.Context().Value("writer").(http.ResponseWriter) // Assuming context passing the ResponseWriter for error utility

	token, err := extractToken(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusUnauthorized, "Missing or invalid authorization token")
		return nil, err
	}

	grpcReq := &pb.ValidateTokenRequest{Token: token}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	validateResp, err := h.AuthClient.ValidateToken(ctx, grpcReq)
	if err != nil {
		handleGRPCError(w, err)
		return nil, err
	}

	if !validateResp.Valid {
		util.WriteJSONError(w, http.StatusUnauthorized, "Invalid or expired session token")
		return nil, errors.New(validateResp.Message)
	}

	// Store the User data in the request context for subsequent handlers to use (e.g., getting the student_id)
	*r = *r.WithContext(context.WithValue(r.Context(), "user", validateResp.User))

	return validateResp, nil
}
