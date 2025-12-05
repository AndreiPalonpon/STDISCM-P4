package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	// Gateway utility package

	"stdiscm_p4/backend/internal/gateway/util"
	pb_admin "stdiscm_p4/backend/internal/pb/admin" // The Admin Service gRPC contract
	pb_auth "stdiscm_p4/backend/internal/pb/auth"   // For context user role checks
)

// AdminHandler holds the gRPC client for the Admin Service.
type AdminHandler struct {
	AdminClient pb_admin.AdminServiceClient
}

// -- Request Structs (Mirroring JSON bodies in REST API Doc) --

type RESTCreateCourseRequest struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Units       int32  `json:"units"`
	Schedule    string `json:"schedule"`
	Room        string `json:"room"`
	Capacity    int32  `json:"capacity"`
	FacultyID   string `json:"faculty_id"`
	Semester    string `json:"semester"`
}

type RESTUpdateCourseRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Units       int32  `json:"units"`
	Schedule    string `json:"schedule"`
	Room        string `json:"room"`
	Capacity    int32  `json:"capacity"`
	FacultyID   string `json:"faculty_id"`
	IsOpen      bool   `json:"is_open"`
}

type RESTAssignFacultyRequest struct {
	FacultyID string `json:"faculty_id"`
}

type RESTCreateUserRequest struct {
	Email      string `json:"email"`
	Role       string `json:"role"`
	Name       string `json:"name"`
	StudentID  string `json:"student_id"`
	FacultyID  string `json:"faculty_id"`
	Department string `json:"department"`
	Major      string `json:"major"`
	YearLevel  int32  `json:"year_level"`
}

type RESTToggleUserStatusRequest struct {
	Activate bool `json:"activate"`
}

type RESTSetEnrollmentPeriodRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type RESTToggleEnrollmentRequest struct {
	Enable bool `json:"enable"`
}

type RESTOverrideEnrollmentRequest struct {
	StudentID string `json:"student_id"`
	CourseID  string `json:"course_id"`
	Reason    string `json:"reason"`
	// Action is determined by the endpoint (/enroll or /drop)
}

type RESTUpdateSystemConfigRequest struct {
	Value string `json:"value"`
}

// -- Helpers --

func getAdminFromContext(r *http.Request) (*pb_auth.User, bool) {
	user, ok := r.Context().Value("user").(*pb_auth.User)
	if !ok || user == nil {
		return nil, false
	}
	return user, user.Role == "admin"
}

// -- Handlers --

// GetSystemStats handles GET /admin/stats
func (h *AdminHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	grpcReq := &pb_admin.GetSystemStatsRequest{}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.GetSystemStats(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Added "success": true to prevent utility wrapper from nesting "stats" under "data"
	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"stats":   grpcResp.Stats,
	})
}

// CreateCourse handles POST /admin/courses
func (h *AdminHandler) CreateCourse(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	var reqBody RESTCreateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.CreateCourseRequest{
		Code:        reqBody.Code,
		Title:       reqBody.Title,
		Description: reqBody.Description,
		Units:       reqBody.Units,
		Schedule:    reqBody.Schedule,
		Room:        reqBody.Room,
		Capacity:    reqBody.Capacity,
		FacultyId:   reqBody.FacultyID,
		Semester:    reqBody.Semester,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.CreateCourse(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check business logic success
	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success":   grpcResp.Success,
		"course_id": grpcResp.CourseId,
		"course":    grpcResp.Course,
		"message":   grpcResp.Message,
	})
}

// UpdateCourse handles PUT /admin/courses/:id
func (h *AdminHandler) UpdateCourse(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	courseID := chi.URLParam(r, "id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Course ID required")
		return
	}

	var reqBody RESTUpdateCourseRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.UpdateCourseRequest{
		CourseId:    courseID,
		Title:       reqBody.Title,
		Description: reqBody.Description,
		Units:       reqBody.Units,
		Schedule:    reqBody.Schedule,
		Room:        reqBody.Room,
		Capacity:    reqBody.Capacity,
		FacultyId:   reqBody.FacultyID,
		IsOpen:      reqBody.IsOpen,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.UpdateCourse(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check business logic success
	if !grpcResp.Success {
		code := http.StatusBadRequest
		if grpcResp.Message == "course not found" {
			code = http.StatusNotFound
		}
		util.WriteJSONError(w, code, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"course":  grpcResp.Course,
		"message": grpcResp.Message,
	})
}

// DeleteCourse handles DELETE /admin/courses/:id
func (h *AdminHandler) DeleteCourse(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	courseID := chi.URLParam(r, "id")

	grpcReq := &pb_admin.DeleteCourseRequest{
		CourseId: courseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.DeleteCourse(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check business logic success
	if !grpcResp.Success {
		code := http.StatusBadRequest
		if grpcResp.Message == "course not found" {
			code = http.StatusNotFound
		}
		util.WriteJSONError(w, code, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}

// AssignFaculty handles POST /admin/courses/:id/assign-faculty
func (h *AdminHandler) AssignFaculty(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	courseID := chi.URLParam(r, "id")
	var reqBody RESTAssignFacultyRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.AssignFacultyRequest{
		CourseId:  courseID,
		FacultyId: reqBody.FacultyID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.AssignFaculty(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check business logic success
	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}

// CreateUser handles POST /admin/users
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	var reqBody RESTCreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.CreateUserRequest{
		Email:      reqBody.Email,
		Role:       reqBody.Role,
		Name:       reqBody.Name,
		StudentId:  reqBody.StudentID,
		FacultyId:  reqBody.FacultyID,
		Department: reqBody.Department,
		Major:      reqBody.Major,
		YearLevel:  reqBody.YearLevel,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.CreateUser(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check business logic success
	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success":          grpcResp.Success,
		"user_id":          grpcResp.UserId,
		"initial_password": grpcResp.InitialPassword,
		"message":          grpcResp.Message,
		"user":             grpcResp.User,
	})
}

// ListUsers handles GET /admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	role := r.URL.Query().Get("role")
	activeOnlyStr := r.URL.Query().Get("active_only")
	activeOnly := false
	if activeOnlyStr != "" {
		if v, err := strconv.ParseBool(activeOnlyStr); err == nil {
			activeOnly = v
		}
	}

	grpcReq := &pb_admin.ListUsersRequest{
		Role:       role,
		ActiveOnly: activeOnly,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.ListUsers(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"users":       grpcResp.Users,
		"total_count": grpcResp.TotalCount,
	})
}

// ResetPassword handles POST /admin/users/:id/reset-password
func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	userID := chi.URLParam(r, "id")

	grpcReq := &pb_admin.ResetPasswordRequest{
		UserId: userID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.ResetPassword(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusNotFound, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      grpcResp.Success,
		"new_password": grpcResp.NewPassword,
		"message":      grpcResp.Message,
	})
}

// ToggleUserStatus handles PATCH /admin/users/:id/status
func (h *AdminHandler) ToggleUserStatus(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	userID := chi.URLParam(r, "id")
	var reqBody RESTToggleUserStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.ToggleUserStatusRequest{
		UserId:   userID,
		Activate: reqBody.Activate,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.ToggleUserStatus(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}

// SetEnrollmentPeriod handles POST /admin/enrollment/period
func (h *AdminHandler) SetEnrollmentPeriod(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	var reqBody RESTSetEnrollmentPeriodRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.SetEnrollmentPeriodRequest{
		StartDate: reqBody.StartDate,
		EndDate:   reqBody.EndDate,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.SetEnrollmentPeriod(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}

// ToggleEnrollment handles POST /admin/enrollment/toggle
func (h *AdminHandler) ToggleEnrollment(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	var reqBody RESTToggleEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.ToggleEnrollmentRequest{
		Enable: reqBody.Enable,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.ToggleEnrollment(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":         grpcResp.Success,
		"enrollment_open": grpcResp.EnrollmentOpen,
		"message":         grpcResp.Message,
	})
}

// OverrideEnrollment handles POST /admin/override/enroll and /admin/override/drop
func (h *AdminHandler) OverrideEnroll(w http.ResponseWriter, r *http.Request) {
	h.handleOverride(w, r, "force_enroll")
}

func (h *AdminHandler) OverrideDrop(w http.ResponseWriter, r *http.Request) {
	h.handleOverride(w, r, "force_drop")
}

func (h *AdminHandler) handleOverride(w http.ResponseWriter, r *http.Request, action string) {
	adminUser, isAdmin := getAdminFromContext(r)
	if !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	var reqBody RESTOverrideEnrollmentRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.OverrideEnrollmentRequest{
		StudentId: reqBody.StudentID,
		CourseId:  reqBody.CourseID,
		Action:    action,
		Reason:    reqBody.Reason,
		AdminId:   adminUser.Id, // Securely taken from authenticated user context
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.OverrideEnrollment(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}

// GetSystemConfig handles GET /admin/config
func (h *AdminHandler) GetSystemConfig(w http.ResponseWriter, r *http.Request) {
	if _, isAdmin := getAdminFromContext(r); !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	key := r.URL.Query().Get("key")

	grpcReq := &pb_admin.GetSystemConfigRequest{
		Key: key,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.GetSystemConfig(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"configs": grpcResp.Configs,
	})
}

// UpdateSystemConfig handles PUT /admin/config/:key
func (h *AdminHandler) UpdateSystemConfig(w http.ResponseWriter, r *http.Request) {
	adminUser, isAdmin := getAdminFromContext(r)
	if !isAdmin {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Admin only")
		return
	}

	key := chi.URLParam(r, "key")

	var reqBody RESTUpdateSystemConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	grpcReq := &pb_admin.UpdateSystemConfigRequest{
		Key:     key,
		Value:   reqBody.Value,
		AdminId: adminUser.Id,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.AdminClient.UpdateSystemConfig(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	util.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": grpcResp.Success,
		"message": grpcResp.Message,
	})
}
