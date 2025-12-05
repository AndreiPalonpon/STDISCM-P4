package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/internal/pb/admin"
	"stdiscm_p4/backend/internal/shared"
)

// AdminService implements the gRPC AdminService
type AdminService struct {
	pb.UnimplementedAdminServiceServer
	client          *mongo.Client
	db              *mongo.Database
	config          *shared.ServiceConfig
	coursesCol      *mongo.Collection
	usersCol        *mongo.Collection
	systemConfigCol *mongo.Collection
	enrollmentsCol  *mongo.Collection
	auditLogsCol    *mongo.Collection
}

// NewAdminService creates a new AdminService instance
func NewAdminService(client *mongo.Client, db *mongo.Database, config *shared.ServiceConfig) *AdminService {
	return &AdminService{
		client:          client,
		db:              db,
		config:          config,
		coursesCol:      db.Collection("courses"),
		usersCol:        db.Collection("users"),
		systemConfigCol: db.Collection("system_config"),
		enrollmentsCol:  db.Collection("enrollments"),
		auditLogsCol:    db.Collection("audit_logs"),
	}
}

// ============================================================================
// Course Management
// ============================================================================

func (s *AdminService) CreateCourse(ctx context.Context, req *pb.CreateCourseRequest) (*pb.CreateCourseResponse, error) {
	if req == nil || req.Code == "" || req.Title == "" || req.Semester == "" {
		return nil, status.Error(codes.InvalidArgument, "code, title, and semester are required")
	}

	if req.Units < 1 || req.Units > 5 {
		return &pb.CreateCourseResponse{Success: false, Message: "units must be between 1 and 5"}, nil
	}
	if req.Capacity < 5 || req.Capacity > 100 {
		return &pb.CreateCourseResponse{Success: false, Message: "capacity must be between 5 and 100"}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check duplicates
	count, err := s.coursesCol.CountDocuments(queryCtx, bson.M{"code": req.Code, "semester": req.Semester})
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	if count > 0 {
		return &pb.CreateCourseResponse{Success: false, Message: fmt.Sprintf("course %s already exists for %s", req.Code, req.Semester)}, nil
	}

	// Verify faculty
	if req.FacultyId != "" {
		if err := s.verifyFaculty(queryCtx, req.FacultyId); err != nil {
			return &pb.CreateCourseResponse{Success: false, Message: "faculty not found"}, nil
		}
	}

	// Use Shared ID generation (Course Code as prefix is fine, but using ID directly is safer)
	courseID := req.Code // Using Code as ID as per original intent, or generate unique?
	// Original code: courseID := generateCourseID(req.Code). Let's use shared.
	courseID = shared.GenerateID(req.Code)

	courseDoc := bson.M{
		"_id":         courseID,
		"code":        req.Code,
		"title":       req.Title,
		"description": req.Description,
		"units":       req.Units,
		"schedule":    req.Schedule,
		"room":        req.Room,
		"capacity":    req.Capacity,
		"enrolled":    0,
		"faculty_id":  req.FacultyId,
		"is_open":     false,
		"semester":    req.Semester,
		"created_at":  primitive.NewDateTimeFromTime(time.Now()),
		"updated_at":  primitive.NewDateTimeFromTime(time.Now()),
	}

	_, err = s.coursesCol.InsertOne(queryCtx, courseDoc)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create course")
	}

	// Log Audit
	shared.LogAuditEvent(queryCtx, s.auditLogsCol, "admin", shared.ActionCourseCreate, courseID, nil)

	return &pb.CreateCourseResponse{
		Success:  true,
		CourseId: courseID,
		Course: &pb.Course{
			Id: courseID, Code: req.Code, Title: req.Title, Description: req.Description,
			Units: req.Units, Schedule: req.Schedule, Room: req.Room, Capacity: req.Capacity,
			FacultyId: req.FacultyId, Semester: req.Semester, IsOpen: false,
		},
		Message: "course created successfully",
	}, nil
}

func (s *AdminService) UpdateCourse(ctx context.Context, req *pb.UpdateCourseRequest) (*pb.UpdateCourseResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check existence and current state
	var existingCourse bson.M
	err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&existingCourse)
	if err == mongo.ErrNoDocuments {
		return &pb.UpdateCourseResponse{Success: false, Message: "course not found"}, nil
	}

	update := bson.M{}
	if req.Title != "" {
		update["title"] = req.Title
	}
	if req.Description != "" {
		update["description"] = req.Description
	}
	if req.Units > 0 {
		update["units"] = req.Units
	}
	if req.Schedule != "" {
		update["schedule"] = req.Schedule
	}
	if req.Room != "" {
		update["room"] = req.Room
	}

	if req.Capacity > 0 {
		currentEnrolled, _ := shared.GetInt32(existingCourse["enrolled"])
		if req.Capacity < currentEnrolled {
			return &pb.UpdateCourseResponse{Success: false, Message: fmt.Sprintf("cannot reduce capacity below current enrollment (%d)", currentEnrolled)}, nil
		}
		update["capacity"] = req.Capacity
	}

	if req.FacultyId != "" {
		if err := s.verifyFaculty(queryCtx, req.FacultyId); err != nil {
			return &pb.UpdateCourseResponse{Success: false, Message: "faculty not found"}, nil
		}
		update["faculty_id"] = req.FacultyId
	}

	update["is_open"] = req.IsOpen
	update["updated_at"] = primitive.NewDateTimeFromTime(time.Now())

	_, err = s.coursesCol.UpdateOne(queryCtx, bson.M{"_id": req.CourseId}, bson.M{"$set": update})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update")
	}

	// Fetch updated
	var updatedDoc bson.M
	s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&updatedDoc)

	shared.LogAuditEvent(queryCtx, s.auditLogsCol, "admin", shared.ActionCourseUpdate, req.CourseId, nil)

	return &pb.UpdateCourseResponse{
		Success: true,
		Course:  s.documentToCourse(updatedDoc),
		Message: "course updated successfully",
	}, nil
}

func (s *AdminService) DeleteCourse(ctx context.Context, req *pb.DeleteCourseRequest) (*pb.DeleteCourseResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check enrollments
	count, err := s.enrollmentsCol.CountDocuments(queryCtx, bson.M{
		"course_id": req.CourseId,
		"status":    bson.M{"$in": []string{shared.StatusEnrolled, shared.StatusCompleted}},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	if count > 0 {
		return &pb.DeleteCourseResponse{Success: false, Message: "cannot delete course with existing enrollments"}, nil
	}

	res, err := s.coursesCol.DeleteOne(queryCtx, bson.M{"_id": req.CourseId})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete")
	}
	if res.DeletedCount == 0 {
		return &pb.DeleteCourseResponse{Success: false, Message: "course not found"}, nil
	}

	return &pb.DeleteCourseResponse{Success: true, Message: "course deleted successfully"}, nil
}

func (s *AdminService) AssignFaculty(ctx context.Context, req *pb.AssignFacultyRequest) (*pb.AssignFacultyResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "args required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.verifyFaculty(queryCtx, req.FacultyId); err != nil {
		return &pb.AssignFacultyResponse{Success: false, Message: "faculty not found or inactive"}, nil
	}

	res, err := s.coursesCol.UpdateOne(queryCtx, bson.M{"_id": req.CourseId}, bson.M{
		"$set": bson.M{"faculty_id": req.FacultyId, "updated_at": time.Now()},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	if res.MatchedCount == 0 {
		return &pb.AssignFacultyResponse{Success: false, Message: "course not found"}, nil
	}

	return &pb.AssignFacultyResponse{Success: true, Message: "faculty assigned successfully"}, nil
}

// ============================================================================
// User Management
// ============================================================================

func (s *AdminService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	if req.Email == "" || req.Role == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "missing fields")
	}
	if !shared.IsValidRole(req.Role) {
		return &pb.CreateUserResponse{Success: false, Message: "invalid role"}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check email
	count, _ := s.usersCol.CountDocuments(queryCtx, bson.M{"email": req.Email})
	if count > 0 {
		return &pb.CreateUserResponse{Success: false, Message: "email exists"}, nil
	}

	// Use Shared ID Gen
	userID := shared.GenerateID(req.Role)

	// Password
	initPwd := s.generateRandomPassword()
	hash, _ := bcrypt.GenerateFromPassword([]byte(initPwd), s.config.Security.BCryptCost)

	userDoc := bson.M{
		"_id": userID, "email": req.Email, "password_hash": string(hash),
		"role": req.Role, "name": req.Name, "is_active": true,
		"created_at": primitive.NewDateTimeFromTime(time.Now()),
	}

	if req.Role == shared.RoleStudent {
		if req.StudentId == "" {
			req.StudentId = shared.GenerateID("STU")
		} // Simplified ID gen
		userDoc["student_id"] = req.StudentId
		userDoc["major"] = req.Major
		userDoc["year_level"] = req.YearLevel
	} else if req.Role == shared.RoleFaculty {
		if req.FacultyId == "" {
			req.FacultyId = shared.GenerateID("FAC")
		}
		userDoc["faculty_id"] = req.FacultyId
		userDoc["department"] = req.Department
	}

	_, err := s.usersCol.InsertOne(queryCtx, userDoc)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	shared.LogAuditEvent(queryCtx, s.auditLogsCol, "admin", shared.ActionUserCreate, userID, nil)

	// Map to proto (simplified)
	return &pb.CreateUserResponse{
		Success: true, UserId: userID, InitialPassword: initPwd,
		Message: "user created",
		User:    &pb.User{Id: userID, Email: req.Email, Name: req.Name, Role: req.Role, IsActive: true},
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if req.Role != "" {
		filter["role"] = req.Role
	}
	if req.ActiveOnly {
		filter["is_active"] = true
	}

	cursor, err := s.usersCol.Find(queryCtx, filter, options.Find().SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(queryCtx)

	var users []*pb.User
	for cursor.Next(queryCtx) {
		var u shared.User
		if err := cursor.Decode(&u); err == nil {
			users = append(users, s.userToProto(&u))
		}
	}
	return &pb.ListUsersResponse{Users: users, TotalCount: int32(len(users))}, nil
}

func (s *AdminService) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "id required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	newPwd := s.generateRandomPassword()
	hash, _ := bcrypt.GenerateFromPassword([]byte(newPwd), s.config.Security.BCryptCost)

	res, err := s.usersCol.UpdateOne(queryCtx, bson.M{"_id": req.UserId}, bson.M{
		"$set": bson.M{"password_hash": string(hash), "updated_at": time.Now()},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	if res.MatchedCount == 0 {
		return &pb.ResetPasswordResponse{Success: false, Message: "user not found"}, nil
	}

	return &pb.ResetPasswordResponse{Success: true, NewPassword: newPwd, Message: "password reset"}, nil
}

func (s *AdminService) ToggleUserStatus(ctx context.Context, req *pb.ToggleUserStatusRequest) (*pb.ToggleUserStatusResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.usersCol.UpdateOne(queryCtx, bson.M{"_id": req.UserId}, bson.M{
		"$set": bson.M{"is_active": req.Activate, "updated_at": time.Now()},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}

	return &pb.ToggleUserStatusResponse{Success: true, Message: "status updated"}, nil
}

// ============================================================================
// System Config
// ============================================================================

func (s *AdminService) SetEnrollmentPeriod(ctx context.Context, req *pb.SetEnrollmentPeriodRequest) (*pb.SetEnrollmentPeriodResponse, error) {
	// Simple passthrough to update config
	s.UpdateSystemConfig(ctx, &pb.UpdateSystemConfigRequest{Key: "enrollment_start", Value: req.StartDate})
	s.UpdateSystemConfig(ctx, &pb.UpdateSystemConfigRequest{Key: "enrollment_end", Value: req.EndDate})
	return &pb.SetEnrollmentPeriodResponse{Success: true, Message: "dates set"}, nil
}

func (s *AdminService) ToggleEnrollment(ctx context.Context, req *pb.ToggleEnrollmentRequest) (*pb.ToggleEnrollmentResponse, error) {
	val := "false"
	if req.Enable {
		val = "true"
	}
	s.UpdateSystemConfig(ctx, &pb.UpdateSystemConfigRequest{Key: "enrollment_enabled", Value: val})
	return &pb.ToggleEnrollmentResponse{Success: true, EnrollmentOpen: req.Enable, Message: "enrollment toggled"}, nil
}

func (s *AdminService) GetSystemConfig(ctx context.Context, req *pb.GetSystemConfigRequest) (*pb.GetSystemConfigResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if req.Key != "" {
		filter["key"] = req.Key
	}
	cursor, err := s.systemConfigCol.Find(queryCtx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(queryCtx)

	var configs []*pb.SystemConfig
	for cursor.Next(queryCtx) {
		var c shared.SystemConfig
		if err := cursor.Decode(&c); err == nil {
			configs = append(configs, &pb.SystemConfig{
				Key: c.Key, Value: c.Value, UpdatedAt: timestamppb.New(c.UpdatedAt), UpdatedBy: c.UpdatedBy,
			})
		}
	}
	return &pb.GetSystemConfigResponse{Configs: configs}, nil
}

func (s *AdminService) UpdateSystemConfig(ctx context.Context, req *pb.UpdateSystemConfigRequest) (*pb.UpdateSystemConfigResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opts := options.Update().SetUpsert(true)
	_, err := s.systemConfigCol.UpdateOne(queryCtx, bson.M{"key": req.Key}, bson.M{
		"$set": bson.M{"value": req.Value, "updated_by": req.AdminId, "updated_at": time.Now()},
	}, opts)
	if err != nil {
		return nil, err
	}

	shared.LogAuditEvent(queryCtx, s.auditLogsCol, req.AdminId, shared.ActionConfigChange, req.Key, nil)
	return &pb.UpdateSystemConfigResponse{Success: true, Message: "updated"}, nil
}

// ============================================================================
// Overrides (Transactions)
// ============================================================================

func (s *AdminService) OverrideEnrollment(ctx context.Context, req *pb.OverrideEnrollmentRequest) (*pb.OverrideEnrollmentResponse, error) {
	if req.Action != "force_enroll" && req.Action != "force_drop" {
		return nil, status.Error(codes.InvalidArgument, "invalid action")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 1. Verify Entities
	var student shared.User
	if err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.StudentId, "role": shared.RoleStudent}).Decode(&student); err != nil {
		return &pb.OverrideEnrollmentResponse{Success: false, Message: "student not found"}, nil
	}

	var course bson.M
	if err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&course); err != nil {
		return &pb.OverrideEnrollmentResponse{Success: false, Message: "course not found"}, nil
	}

	// 2. Perform Transaction using Shared Helper
	err := shared.WithTransaction(queryCtx, s.client, func(sessCtx mongo.SessionContext) error {
		if req.Action == "force_enroll" {
			// Check existing
			count, _ := s.enrollmentsCol.CountDocuments(sessCtx, bson.M{"student_id": req.StudentId, "course_id": req.CourseId, "status": shared.StatusEnrolled})
			if count > 0 {
				return fmt.Errorf("already enrolled")
			}

			// Create Enrollment
			enrollmentID := shared.GenerateEnrollmentID()
			scheduleInfo := shared.ExtractScheduleInfo(course) // Use Shared Helper

			_, err := s.enrollmentsCol.InsertOne(sessCtx, bson.M{
				"_id": enrollmentID, "student_id": req.StudentId, "course_id": req.CourseId,
				"status": shared.StatusEnrolled, "enrolled_at": time.Now(), "schedule_info": scheduleInfo,
			})
			if err != nil {
				return err
			}

			// Inc Course
			if _, err := s.coursesCol.UpdateOne(sessCtx, bson.M{"_id": req.CourseId}, bson.M{"$inc": bson.M{"enrolled": 1}}); err != nil {
				return err
			}

		} else { // force_drop
			res, err := s.enrollmentsCol.UpdateOne(sessCtx,
				bson.M{"student_id": req.StudentId, "course_id": req.CourseId, "status": shared.StatusEnrolled},
				bson.M{"$set": bson.M{"status": shared.StatusDropped, "dropped_at": time.Now()}},
			)
			if err != nil {
				return err
			}
			if res.MatchedCount == 0 {
				return fmt.Errorf("enrollment not found")
			}

			// Dec Course
			if _, err := s.coursesCol.UpdateOne(sessCtx, bson.M{"_id": req.CourseId}, bson.M{"$inc": bson.M{"enrolled": -1}}); err != nil {
				return err
			}
		}

		shared.LogAuditEvent(sessCtx, s.auditLogsCol, req.AdminId, req.Action, fmt.Sprintf("%s:%s", req.StudentId, req.CourseId), nil)
		return nil
	})

	if err != nil {
		return &pb.OverrideEnrollmentResponse{Success: false, Message: err.Error()}, nil
	}

	return &pb.OverrideEnrollmentResponse{Success: true, Message: "override successful"}, nil
}

// ============================================================================
// Stats
// ============================================================================

func (s *AdminService) GetSystemStats(ctx context.Context, req *pb.GetSystemStatsRequest) (*pb.GetSystemStatsResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stats := &pb.SystemStats{}

	// Use shared.CountDocumentsWithTimeout if available, or just standard count
	stCount, _ := s.usersCol.CountDocuments(queryCtx, bson.M{"role": shared.RoleStudent})
	facCount, _ := s.usersCol.CountDocuments(queryCtx, bson.M{"role": shared.RoleFaculty})
	crsCount, _ := s.coursesCol.CountDocuments(queryCtx, bson.M{})
	openCount, _ := s.coursesCol.CountDocuments(queryCtx, bson.M{"is_open": true})
	enrCount, _ := s.enrollmentsCol.CountDocuments(queryCtx, bson.M{"status": shared.StatusEnrolled})

	stats.TotalStudents = int32(stCount)
	stats.TotalFaculty = int32(facCount)
	stats.TotalCourses = int32(crsCount)
	stats.OpenCourses = int32(openCount)
	stats.TotalEnrollments = int32(enrCount)

	return &pb.GetSystemStatsResponse{Stats: stats}, nil
}

// ============================================================================
// Helpers
// ============================================================================

func (s *AdminService) verifyFaculty(ctx context.Context, id string) error {
	res := s.usersCol.FindOne(ctx, bson.M{"_id": id, "role": shared.RoleFaculty, "is_active": true})
	return res.Err()
}

func (s *AdminService) generateRandomPassword() string {
	b := make([]byte, 8)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *AdminService) documentToCourse(doc bson.M) *pb.Course {
	// Simplified mapper
	c := &pb.Course{}
	if v, _ := shared.GetString(doc["_id"]); v != "" {
		c.Id = v
	}
	if v, _ := shared.GetString(doc["code"]); v != "" {
		c.Code = v
	}
	if v, _ := shared.GetString(doc["title"]); v != "" {
		c.Title = v
	}
	if v, _ := shared.GetString(doc["description"]); v != "" {
		c.Description = v
	}
	if v, _ := shared.GetInt32(doc["units"]); v > 0 {
		c.Units = v
	}
	if v, _ := shared.GetString(doc["schedule"]); v != "" {
		c.Schedule = v
	}
	if v, _ := shared.GetString(doc["room"]); v != "" {
		c.Room = v
	}
	if v, _ := shared.GetInt32(doc["capacity"]); v > 0 {
		c.Capacity = v
	}
	if v, _ := shared.GetInt32(doc["enrolled"]); v >= 0 {
		c.Enrolled = v
	}
	if v, _ := shared.GetString(doc["faculty_id"]); v != "" {
		c.FacultyId = v
	}
	if v, _ := shared.GetBool(doc["is_open"]); true {
		c.IsOpen = v
	}
	if v, _ := shared.GetString(doc["semester"]); v != "" {
		c.Semester = v
	}
	return c
}

func (s *AdminService) userToProto(u *shared.User) *pb.User {
	return &pb.User{
		Id: u.ID, Email: u.Email, Role: u.Role, Name: u.Name,
		StudentId: u.StudentID, FacultyId: u.FacultyID, IsActive: u.IsActive,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}
