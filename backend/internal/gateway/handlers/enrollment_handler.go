package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"stdiscm_p4/backend/internal/gateway/util"                // Gateway utility package
	pb_auth "stdiscm_p4/backend/internal/pb/auth"             // To retrieve student_id from context
	pb_enrollment "stdiscm_p4/backend/internal/pb/enrollment" // The Enrollment Service gRPC contract
)

// EnrollmentHandler holds the gRPC client for the Enrollment Service.
type EnrollmentHandler struct {
	EnrollmentClient pb_enrollment.EnrollmentServiceClient
}

// RESTAddToCartRequest mirrors the JSON input for POST /cart/add
type RESTAddToCartRequest struct {
	CourseID string `json:"course_id"`
}

// RESTDropCourseRequest mirrors the JSON input for POST /enrollment/drop
type RESTDropCourseRequest struct {
	CourseID string `json:"course_id"`
}

// Helper to get student_id from context
func getStudentID(r *http.Request) (string, error) {
	user, ok := r.Context().Value("user").(*pb_auth.User)
	if !ok || user == nil {
		return "", http.ErrNoCookie // specific error not important, just indicates missing
	}
	if user.Role != "student" {
		return "", http.ErrNoCookie // indicates invalid role for this action
	}
	return user.StudentId, nil
}

// GetCart handles GET /cart
func (h *EnrollmentHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only students have shopping carts")
		return
	}

	grpcReq := &pb_enrollment.GetCartRequest{
		StudentId: studentID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.GetCart(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// According to REST doc, respond with { success: true, cart: {...} }
	response := map[string]interface{}{
		"success": true,
		"cart":    grpcResp.Cart,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// AddToCart handles POST /cart/add
func (h *EnrollmentHandler) AddToCart(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only students can add to cart")
		return
	}

	var reqBody RESTAddToCartRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if reqBody.CourseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	grpcReq := &pb_enrollment.AddToCartRequest{
		StudentId: studentID,
		CourseId:  reqBody.CourseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.AddToCart(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		// Business logic failure (e.g., cart full, course not found)
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": grpcResp.Message,
		"cart":    grpcResp.Cart,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// RemoveFromCart handles DELETE /cart/remove/:course_id
func (h *EnrollmentHandler) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	courseID := chi.URLParam(r, "course_id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	grpcReq := &pb_enrollment.RemoveFromCartRequest{
		StudentId: studentID,
		CourseId:  courseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.RemoveFromCart(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": grpcResp.Message,
		"cart":    grpcResp.Cart,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// ClearCart handles DELETE /cart/clear
func (h *EnrollmentHandler) ClearCart(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	grpcReq := &pb_enrollment.ClearCartRequest{
		StudentId: studentID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.ClearCart(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": grpcResp.Message,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// EnrollAll handles POST /enrollment/enroll-all
func (h *EnrollmentHandler) EnrollAll(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	grpcReq := &pb_enrollment.EnrollAllRequest{
		StudentId: studentID,
	}

	// Enrollment might take slightly longer due to transactional checks
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.EnrollAll(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		// This could contain partial failures or a total rollback
		response := map[string]interface{}{
			"success":        false,
			"message":        grpcResp.Message,
			"failed_courses": grpcResp.FailedCourses,
		}
		util.WriteJSON(w, http.StatusConflict, response)
		return
	}

	response := map[string]interface{}{
		"success":        true,
		"message":        grpcResp.Message,
		"enrollments":    grpcResp.Enrollments,
		"failed_courses": grpcResp.FailedCourses,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// DropCourse handles POST /enrollment/drop
func (h *EnrollmentHandler) DropCourse(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	var reqBody RESTDropCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if reqBody.CourseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	grpcReq := &pb_enrollment.DropCourseRequest{
		StudentId: studentID,
		CourseId:  reqBody.CourseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.DropCourse(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": grpcResp.Message,
	}
	util.WriteJSON(w, http.StatusOK, response)
}

// GetStudentEnrollments handles GET /enrollment/schedule
func (h *EnrollmentHandler) GetStudentEnrollments(w http.ResponseWriter, r *http.Request) {
	studentID, err := getStudentID(r)
	if err != nil {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied")
		return
	}

	// Extract query params for filtering
	semester := r.URL.Query().Get("semester")
	status := r.URL.Query().Get("status") // optional: enrolled, dropped, completed

	grpcReq := &pb_enrollment.GetStudentEnrollmentsRequest{
		StudentId: studentID,
		Semester:  semester,
		Status:    status,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.EnrollmentClient.GetStudentEnrollments(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Added "success": true
	response := map[string]interface{}{
		"success":     true,
		"enrollments": grpcResp.Enrollments,
		"total_units": grpcResp.TotalUnits,
	}
	util.WriteJSON(w, http.StatusOK, response)
}
