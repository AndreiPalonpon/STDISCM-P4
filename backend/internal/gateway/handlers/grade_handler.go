package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"stdiscm_p4/backend/internal/gateway/util"      // Gateway utility package
	pb_auth "stdiscm_p4/backend/internal/pb/auth"   // For context user role checks
	pb_grade "stdiscm_p4/backend/internal/pb/grade" // The Grade Service gRPC contract
)

// GradeHandler holds the gRPC client for the Grade Service.
type GradeHandler struct {
	GradeClient pb_grade.GradeServiceClient
}

// RESTUploadGradesRequest mirrors the JSON input for POST /grades/upload/:course_id
type RESTUploadGradesRequest struct {
	Entries []RESTGradeEntry `json:"entries"`
}

type RESTGradeEntry struct {
	StudentID string `json:"student_id"`
	Grade     string `json:"grade"`
}

// helper to get user from context
func getUserFromContext(r *http.Request) *pb_auth.User {
	user, ok := r.Context().Value("user").(*pb_auth.User)
	if !ok {
		return nil
	}
	return user
}

// GetStudentGrades handles GET /grades
// Retrieves grades for the logged-in student.
// Query Params: semester (optional)
func (h *GradeHandler) GetStudentGrades(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is a student
	user := getUserFromContext(r)
	if user == nil || user.Role != "student" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only students can view their own grades")
		return
	}

	// 2. Extract Query Parameters
	semester := r.URL.Query().Get("semester")

	// 3. Prepare gRPC Request
	grpcReq := &pb_grade.GetStudentGradesRequest{
		StudentId: user.StudentId, // Trusting the token's student ID
		Semester:  semester,
	}

	// 4. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.GradeClient.GetStudentGrades(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success":  true,
		"grades":   grpcResp.Grades,
		"gpa_info": grpcResp.GpaInfo,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// CalculateGPA handles GET /grades/gpa
// Explicitly calculates GPA for the student (useful if separate from GetStudentGrades).
// Query Params: semester (optional)
func (h *GradeHandler) CalculateGPA(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is a student
	user := getUserFromContext(r)
	if user == nil || user.Role != "student" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only students can calculate their GPA")
		return
	}

	// 2. Extract Query Parameters
	semester := r.URL.Query().Get("semester")

	// 3. Prepare gRPC Request
	grpcReq := &pb_grade.CalculateGPARequest{
		StudentId: user.StudentId,
		Semester:  semester,
	}

	// 4. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.GradeClient.CalculateGPA(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success":  grpcResp.Success,
		"gpa_info": grpcResp.GpaInfo,
		"message":  grpcResp.Message,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// GetClassRoster handles GET /grades/roster/:course_id
// Retrieves the class roster for a specific course (Faculty only).
func (h *GradeHandler) GetClassRoster(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is faculty
	user := getUserFromContext(r)
	if user == nil || user.Role != "faculty" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only faculty can view rosters")
		return
	}

	// 2. Extract Path Variable
	courseID := chi.URLParam(r, "course_id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	// 3. Prepare gRPC Request
	grpcReq := &pb_grade.GetClassRosterRequest{
		CourseId: courseID,
	}

	// 4. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.GradeClient.GetClassRoster(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success":        true,
		"course_id":      grpcResp.CourseId,
		"course_code":    grpcResp.CourseCode,
		"course_title":   grpcResp.CourseTitle,
		"students":       grpcResp.Students,
		"total_students": grpcResp.TotalStudents,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// GetCourseGrades handles GET /grades/course/:course_id
// Retrieves all grades uploaded for a specific course (Faculty only).
func (h *GradeHandler) GetCourseGrades(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is faculty
	user := getUserFromContext(r)
	if user == nil || user.Role != "faculty" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only faculty can view course grades")
		return
	}

	// 2. Extract Path Variable
	courseID := chi.URLParam(r, "course_id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	// 3. Prepare gRPC Request
	// FIX: Use user.Id (System ID) instead of user.FacultyId (Business ID) for DB lookups
	grpcReq := &pb_grade.GetCourseGradesRequest{
		CourseId:  courseID,
		FacultyId: user.Id,
	}

	// 4. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.GradeClient.GetCourseGrades(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success":       true,
		"grades":        grpcResp.Grades,
		"total_grades":  grpcResp.TotalGrades,
		"all_published": grpcResp.AllPublished,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// UploadGrades handles POST /grades/upload/:course_id
// Uploads a batch of grades using client-side streaming with a specialized first message.
func (h *GradeHandler) UploadGrades(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is faculty
	user := getUserFromContext(r)
	if user == nil || user.Role != "faculty" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only faculty can upload grades")
		return
	}

	// 2. Extract Path and Body
	courseID := chi.URLParam(r, "course_id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	var reqBody RESTUploadGradesRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		util.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(reqBody.Entries) == 0 {
		util.WriteJSONError(w, http.StatusBadRequest, "No grade entries provided")
		return
	}

	// 3. Initiate Stream
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second) // Longer timeout for bulk uploads
	defer cancel()

	stream, err := h.GradeClient.UploadGrades(ctx)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 4. Send Metadata (First Message)
	// We use the oneof Payload field to send Metadata first
	// FIX: Use user.Id (System ID) so the service can validate against the DB _id
	metaReq := &pb_grade.UploadGradeEntryRequest{
		Payload: &pb_grade.UploadGradeEntryRequest_Metadata{
			Metadata: &pb_grade.UploadMetadata{
				CourseId:  courseID,
				FacultyId: user.Id,
			},
		},
		IsLast: false,
	}

	if err := stream.Send(metaReq); err != nil {
		util.WriteJSONError(w, http.StatusInternalServerError, "Failed to stream metadata: "+err.Error())
		return
	}

	// 5. Send Grade Entries
	for i, entry := range reqBody.Entries {
		isLast := i == len(reqBody.Entries)-1

		req := &pb_grade.UploadGradeEntryRequest{
			Payload: &pb_grade.UploadGradeEntryRequest_Entry{
				Entry: &pb_grade.GradeEntry{
					StudentId: entry.StudentID,
					Grade:     entry.Grade,
				},
			},
			IsLast: isLast,
		}

		if err := stream.Send(req); err != nil {
			util.WriteJSONError(w, http.StatusInternalServerError, "Failed to stream grade entry: "+err.Error())
			return
		}
	}

	// 6. Close and Receive Response
	grpcResp, err := stream.CloseAndRecv()
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// 7. Map and Respond
	response := map[string]interface{}{
		"success":         grpcResp.Success,
		"total_processed": grpcResp.TotalProcessed,
		"successful":      grpcResp.Successful,
		"failed":          grpcResp.Failed,
		"errors":          grpcResp.Errors,
		"message":         grpcResp.Message,
	}

	util.WriteJSON(w, http.StatusOK, response)
}

// PublishGrades handles POST /grades/publish/:course_id
// Makes uploaded grades visible to students.
func (h *GradeHandler) PublishGrades(w http.ResponseWriter, r *http.Request) {
	// 1. Authorization: Verify user is faculty
	user := getUserFromContext(r)
	if user == nil || user.Role != "faculty" {
		util.WriteJSONError(w, http.StatusForbidden, "Access denied: Only faculty can publish grades")
		return
	}

	// 2. Extract Path Variable
	courseID := chi.URLParam(r, "course_id")
	if courseID == "" {
		util.WriteJSONError(w, http.StatusBadRequest, "course_id is required")
		return
	}

	// 3. Prepare gRPC Request
	// FIX: Use user.Id (System ID) for validation
	grpcReq := &pb_grade.PublishGradesRequest{
		CourseId:  courseID,
		FacultyId: user.Id,
	}

	// 4. Call gRPC Service
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	grpcResp, err := h.GradeClient.PublishGrades(ctx, grpcReq)
	if err != nil {
		util.HandleGRPCError(w, err)
		return
	}

	// FIX: Check for business logic failure (e.g. faculty validation failed inside service)
	if !grpcResp.Success {
		util.WriteJSONError(w, http.StatusBadRequest, grpcResp.Message)
		return
	}

	// 5. Map and Respond
	response := map[string]interface{}{
		"success":          grpcResp.Success,
		"grades_published": grpcResp.GradesPublished,
		"message":          grpcResp.Message,
	}

	util.WriteJSON(w, http.StatusOK, response)
}
