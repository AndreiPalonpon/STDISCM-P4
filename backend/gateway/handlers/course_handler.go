package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"stdiscm_p4/backend/gateway/util"        // Gateway utility package
	pb_course "stdiscm_p4/backend/pb/course" // The Course Service gRPC contract
)

// CourseHandler holds the gRPC client for the Course Service.
type CourseHandler struct {
	CourseClient pb_course.CourseServiceClient
}

// ListCourses handles GET /courses
// Query Params: department, search, open_only (bool), semester
func (h *CourseHandler) ListCourses(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Query Parameters
	query := r.URL.Query()
	department := query.Get("department")
	searchQuery := query.Get("search")
	semester := query.Get("semester")
	openOnlyStr := query.Get("open_only")

	// Convert open_only string to boolean
	openOnly := false
	if openOnlyStr != "" {
		if val, err := strconv.ParseBool(openOnlyStr); err == nil {
			openOnly = val
		}
	}

	// 2. Prepare gRPC Request
	grpcReq := &pb_course.ListCoursesRequest{
		Filters: &pb_course.CourseFilter{
			Department:  department,
			SearchQuery: searchQuery,
			OpenOnly:    openOnly,
			Semester:    semester,
		},
	}

	// 3. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.CourseClient.ListCourses(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 4. Map and Respond
	response := map[string]interface{}{
		"courses":     grpcResp.Courses,
		"total_count": grpcResp.TotalCount,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// GetCourse handles GET /courses/:id
// Gets detailed information for a specific course.
func (h *CourseHandler) GetCourse(w http.ResponseWriter, r *http.Request) {
	// 1. Extract Path Variable
	courseID := chi.URLParam(r, "id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Course ID is required")
		return
	}

	// 2. Prepare gRPC Request
	grpcReq := &pb_course.GetCourseRequest{
		CourseId: courseID,
	}

	// 3. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.CourseClient.GetCourse(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 4. Handle Business Logic Failure
	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusNotFound, grpcResp.Message)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success": true,
		"course":  grpcResp.Course,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// GetCourseAvailability handles GET /courses/:id/availability
// Checks real-time seat availability.
func (h *CourseHandler) GetCourseAvailability(w http.ResponseWriter, r *http.Request) {
	courseID := chi.URLParam(r, "id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Course ID is required")
		return
	}

	grpcReq := &pb_course.GetCourseAvailabilityRequest{
		CourseId: courseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.CourseClient.GetCourseAvailability(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// Map gRPC response to REST format
	response := map[string]interface{}{
		"available":       grpcResp.Available,
		"capacity":        grpcResp.Capacity,
		"enrolled":        grpcResp.Enrolled,
		"seats_remaining": grpcResp.SeatsRemaining,
		"is_open":         grpcResp.IsOpen,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// CheckPrerequisites handles GET /courses/:id/prerequisites
// Query Params: student_id
func (h *CourseHandler) CheckPrerequisites(w http.ResponseWriter, r *http.Request) {
	courseID := chi.URLParam(r, "id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "Course ID is required")
		return
	}

	// Per REST API doc, student_id is passed as a query parameter
	studentID := r.URL.Query().Get("student_id")
	if studentID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "student_id query parameter is required")
		return
	}

	grpcReq := &pb_course.CheckPrerequisitesRequest{
		StudentId: studentID,
		CourseId:  courseID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.CourseClient.CheckPrerequisites(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	response := map[string]interface{}{
		"all_met":       grpcResp.AllMet,
		"prerequisites": grpcResp.Prerequisites,
	}

	util.WriteJSON(w, http.StatusOK, response)
}
