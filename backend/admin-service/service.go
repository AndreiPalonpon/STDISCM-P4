package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/pb/admin"
)

// AdminService implements the gRPC AdminService
type AdminService struct {
	pb.UnimplementedAdminServiceServer
	db             *mongo.Database
	coursesCol     *mongo.Collection
	usersCol       *mongo.Collection
	systemConfigCol *mongo.Collection
	enrollmentsCol *mongo.Collection
	auditLogsCol   *mongo.Collection
}

// NewAdminService creates a new AdminService instance
func NewAdminService(db *mongo.Database) *AdminService {
	return &AdminService{
		db:             db,
		coursesCol:     db.Collection("courses"),
		usersCol:       db.Collection("users"),
		systemConfigCol: db.Collection("system_config"),
		enrollmentsCol: db.Collection("enrollments"),
		auditLogsCol:   db.Collection("audit_logs"),
	}
}

// ============================================================================
// Course Management
// ============================================================================

// CreateCourse creates a new course
func (s *AdminService) CreateCourse(ctx context.Context, req *pb.CreateCourseRequest) (*pb.CreateCourseResponse, error) {
	if req == nil || req.Code == "" || req.Title == "" || req.Semester == "" {
		return nil, status.Error(codes.InvalidArgument, "code, title, and semester are required")
	}

	// Validate units
	if req.Units < 1 || req.Units > 5 {
		return &pb.CreateCourseResponse{
			Success: false,
			Message: "units must be between 1 and 5",
		}, nil
	}

	// Validate capacity
	if req.Capacity < 5 || req.Capacity > 100 {
		return &pb.CreateCourseResponse{
			Success: false,
			Message: "capacity must be between 5 and 100",
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if course code already exists for the semester
	existingCount, err := s.coursesCol.CountDocuments(queryCtx, bson.M{
		"code":     req.Code,
		"semester": req.Semester,
	})
	if err != nil {
		log.Printf("Error checking existing course: %v", err)
		return nil, status.Error(codes.Internal, "failed to check existing course")
	}
	if existingCount > 0 {
		return &pb.CreateCourseResponse{
			Success: false,
			Message: fmt.Sprintf("course %s already exists for %s", req.Code, req.Semester),
		}, nil
	}

	// Verify faculty exists if provided
	if req.FacultyId != "" {
		var faculty bson.M
		err := s.usersCol.FindOne(queryCtx, bson.M{
			"_id":  req.FacultyId,
			"role": "faculty",
		}).Decode(&faculty)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return &pb.CreateCourseResponse{
					Success: false,
					Message: "faculty not found",
				}, nil
			}
			log.Printf("Error verifying faculty: %v", err)
			return nil, status.Error(codes.Internal, "failed to verify faculty")
		}
	}

	// Generate course ID
	courseID := generateCourseID(req.Code)

	// Create course document
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
		"is_open":     false, // Default to closed
		"semester":    req.Semester,
		"created_at":  primitive.NewDateTimeFromTime(time.Now()),
		"updated_at":  primitive.NewDateTimeFromTime(time.Now()),
	}

	_, err = s.coursesCol.InsertOne(queryCtx, courseDoc)
	if err != nil {
		log.Printf("Error creating course: %v", err)
		return nil, status.Error(codes.Internal, "failed to create course")
	}

	// Build course response
	course := &pb.Course{
		Id:          courseID,
		Code:        req.Code,
		Title:       req.Title,
		Description: req.Description,
		Units:       req.Units,
		Schedule:    req.Schedule,
		Room:        req.Room,
		Capacity:    req.Capacity,
		Enrolled:    0,
		FacultyId:   req.FacultyId,
		IsOpen:      false,
		Semester:    req.Semester,
	}

	log.Printf("Course created successfully: %s (%s)", req.Code, courseID)

	return &pb.CreateCourseResponse{
		Success:  true,
		CourseId: courseID,
		Course:   course,
		Message:  "course created successfully",
	}, nil
}

// UpdateCourse updates an existing course
func (s *AdminService) UpdateCourse(ctx context.Context, req *pb.UpdateCourseRequest) (*pb.UpdateCourseResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if course exists
	var existingCourse bson.M
	err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&existingCourse)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.UpdateCourseResponse{
				Success: false,
				Message: "course not found",
			}, nil
		}
		log.Printf("Error finding course: %v", err)
		return nil, status.Error(codes.Internal, "failed to find course")
	}

	// Build update document
	update := bson.M{}

	if req.Title != "" {
		update["title"] = req.Title
	}
	if req.Description != "" {
		update["description"] = req.Description
	}
	if req.Units > 0 {
		if req.Units < 1 || req.Units > 5 {
			return &pb.UpdateCourseResponse{
				Success: false,
				Message: "units must be between 1 and 5",
			}, nil
		}
		update["units"] = req.Units
	}
	if req.Schedule != "" {
		update["schedule"] = req.Schedule
	}
	if req.Room != "" {
		update["room"] = req.Room
	}
	if req.Capacity > 0 {
		if req.Capacity < 5 || req.Capacity > 100 {
			return &pb.UpdateCourseResponse{
				Success: false,
				Message: "capacity must be between 5 and 100",
			}, nil
		}
		// Check if new capacity is less than current enrollment
		currentEnrolled := int32(0)
		if enrolled, ok := existingCourse["enrolled"].(int32); ok {
			currentEnrolled = enrolled
		} else if enrolled, ok := existingCourse["enrolled"].(int64); ok {
			currentEnrolled = int32(enrolled)
		}
		if req.Capacity < currentEnrolled {
			return &pb.UpdateCourseResponse{
				Success: false,
				Message: fmt.Sprintf("cannot reduce capacity below current enrollment (%d)", currentEnrolled),
			}, nil
		}
		update["capacity"] = req.Capacity
	}
	if req.FacultyId != "" {
		// Verify faculty exists
		var faculty bson.M
		err := s.usersCol.FindOne(queryCtx, bson.M{
			"_id":  req.FacultyId,
			"role": "faculty",
		}).Decode(&faculty)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return &pb.UpdateCourseResponse{
					Success: false,
					Message: "faculty not found",
				}, nil
			}
			log.Printf("Error verifying faculty: %v", err)
			return nil, status.Error(codes.Internal, "failed to verify faculty")
		}
		update["faculty_id"] = req.FacultyId
	}

	// Update is_open status
	update["is_open"] = req.IsOpen
	update["updated_at"] = primitive.NewDateTimeFromTime(time.Now())

	// Perform update
	result, err := s.coursesCol.UpdateOne(
		queryCtx,
		bson.M{"_id": req.CourseId},
		bson.M{"$set": update},
	)
	if err != nil {
		log.Printf("Error updating course: %v", err)
		return nil, status.Error(codes.Internal, "failed to update course")
	}

	if result.MatchedCount == 0 {
		return &pb.UpdateCourseResponse{
			Success: false,
			Message: "course not found",
		}, nil
	}

	// Fetch updated course
	var updatedDoc bson.M
	err = s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&updatedDoc)
	if err != nil {
		log.Printf("Error fetching updated course: %v", err)
		return nil, status.Error(codes.Internal, "failed to fetch updated course")
	}

	course := documentToCourse(updatedDoc)

	log.Printf("Course updated successfully: %s", req.CourseId)

	return &pb.UpdateCourseResponse{
		Success: true,
		Course:  course,
		Message: "course updated successfully",
	}, nil
}

// DeleteCourse deletes a course (only if no enrollments)
func (s *AdminService) DeleteCourse(ctx context.Context, req *pb.DeleteCourseRequest) (*pb.DeleteCourseResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if course has any enrollments
	enrollmentCount, err := s.enrollmentsCol.CountDocuments(queryCtx, bson.M{
		"course_id": req.CourseId,
		"status":    bson.M{"$in": []string{"enrolled", "completed"}},
	})
	if err != nil {
		log.Printf("Error checking enrollments: %v", err)
		return nil, status.Error(codes.Internal, "failed to check enrollments")
	}

	if enrollmentCount > 0 {
		return &pb.DeleteCourseResponse{
			Success: false,
			Message: fmt.Sprintf("cannot delete course with existing enrollments (%d)", enrollmentCount),
		}, nil
	}

	// Delete the course
	result, err := s.coursesCol.DeleteOne(queryCtx, bson.M{"_id": req.CourseId})
	if err != nil {
		log.Printf("Error deleting course: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete course")
	}

	if result.DeletedCount == 0 {
		return &pb.DeleteCourseResponse{
			Success: false,
			Message: "course not found",
		}, nil
	}

	log.Printf("Course deleted successfully: %s", req.CourseId)

	return &pb.DeleteCourseResponse{
		Success: true,
		Message: "course deleted successfully",
	}, nil
}

// AssignFaculty assigns a faculty member to a course
func (s *AdminService) AssignFaculty(ctx context.Context, req *pb.AssignFacultyRequest) (*pb.AssignFacultyResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id and faculty_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Verify faculty exists
	var faculty bson.M
	err := s.usersCol.FindOne(queryCtx, bson.M{
		"_id":       req.FacultyId,
		"role":      "faculty",
		"is_active": true,
	}).Decode(&faculty)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.AssignFacultyResponse{
				Success: false,
				Message: "faculty not found or inactive",
			}, nil
		}
		log.Printf("Error verifying faculty: %v", err)
		return nil, status.Error(codes.Internal, "failed to verify faculty")
	}

	// Update course with faculty assignment
	result, err := s.coursesCol.UpdateOne(
		queryCtx,
		bson.M{"_id": req.CourseId},
		bson.M{
			"$set": bson.M{
				"faculty_id": req.FacultyId,
				"updated_at": primitive.NewDateTimeFromTime(time.Now()),
			},
		},
	)
	if err != nil {
		log.Printf("Error assigning faculty: %v", err)
		return nil, status.Error(codes.Internal, "failed to assign faculty")
	}

	if result.MatchedCount == 0 {
		return &pb.AssignFacultyResponse{
			Success: false,
			Message: "course not found",
		}, nil
	}

	log.Printf("Faculty assigned successfully: %s to course %s", req.FacultyId, req.CourseId)

	return &pb.AssignFacultyResponse{
		Success: true,
		Message: "faculty assigned successfully",
	}, nil
}

// ============================================================================
// User Management
// ============================================================================

// CreateUser creates a new user account
func (s *AdminService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	if req == nil || req.Email == "" || req.Role == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "email, role, and name are required")
	}

	// Validate role
	if req.Role != "student" && req.Role != "faculty" && req.Role != "admin" {
		return &pb.CreateUserResponse{
			Success: false,
			Message: "role must be 'student', 'faculty', or 'admin'",
		}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if email already exists
	existingCount, err := s.usersCol.CountDocuments(queryCtx, bson.M{"email": req.Email})
	if err != nil {
		log.Printf("Error checking existing email: %v", err)
		return nil, status.Error(codes.Internal, "failed to check existing email")
	}
	if existingCount > 0 {
		return &pb.CreateUserResponse{
			Success: false,
			Message: "email already exists",
		}, nil
	}

	// Generate user ID
	userID := generateUserID(req.Role)

	// Generate initial password
	initialPassword := generateRandomPassword()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate password")
	}

	// Build user document
	userDoc := bson.M{
		"_id":           userID,
		"email":         req.Email,
		"password_hash": string(passwordHash),
		"role":          req.Role,
		"name":          req.Name,
		"is_active":     true,
		"created_at":    primitive.NewDateTimeFromTime(time.Now()),
	}

	// Add role-specific fields
	if req.Role == "student" {
		if req.StudentId == "" {
			req.StudentId = generateStudentID()
		}
		// Check if student_id already exists
		existingCount, err := s.usersCol.CountDocuments(queryCtx, bson.M{"student_id": req.StudentId})
		if err == nil && existingCount > 0 {
			return &pb.CreateUserResponse{
				Success: false,
				Message: "student_id already exists",
			}, nil
		}
		userDoc["student_id"] = req.StudentId
		userDoc["major"] = req.Major
		userDoc["year_level"] = req.YearLevel
	} else if req.Role == "faculty" {
		if req.FacultyId == "" {
			req.FacultyId = generateFacultyID()
		}
		// Check if faculty_id already exists
		existingCount, err := s.usersCol.CountDocuments(queryCtx, bson.M{"faculty_id": req.FacultyId})
		if err == nil && existingCount > 0 {
			return &pb.CreateUserResponse{
				Success: false,
				Message: "faculty_id already exists",
			}, nil
		}
		userDoc["faculty_id"] = req.FacultyId
		userDoc["department"] = req.Department
	}

	// Insert user
	_, err = s.usersCol.InsertOne(queryCtx, userDoc)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	// Build user response
	user := &pb.User{
		Id:         userID,
		Email:      req.Email,
		Role:       req.Role,
		Name:       req.Name,
		StudentId:  req.StudentId,
		FacultyId:  req.FacultyId,
		Department: req.Department,
		Major:      req.Major,
		YearLevel:  req.YearLevel,
		IsActive:   true,
		CreatedAt:  timestamppb.Now(),
	}

	log.Printf("User created successfully: %s (%s)", req.Email, userID)

	return &pb.CreateUserResponse{
		Success:         true,
		UserId:          userID,
		InitialPassword: initialPassword,
		User:            user,
		Message:         "user created successfully",
	}, nil
}

// ListUsers lists all users with optional filtering
func (s *AdminService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build filter
	filter := bson.M{}
	if req != nil {
		if req.Role != "" {
			filter["role"] = req.Role
		}
		if req.ActiveOnly {
			filter["is_active"] = true
		}
	}

	// Query users
	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(1000) // Reasonable limit

	cursor, err := s.usersCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying users: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve users")
	}
	defer cursor.Close(queryCtx)

	var users []*pb.User
	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding user document: %v", err)
			continue
		}

		user := documentToUser(doc)
		users = append(users, user)
	}

	return &pb.ListUsersResponse{
		Users:      users,
		TotalCount: int32(len(users)),
	}, nil
}

// ResetPassword resets a user's password
func (s *AdminService) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if user exists
	var user bson.M
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.UserId}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.ResetPasswordResponse{
				Success: false,
				Message: "user not found",
			}, nil
		}
		log.Printf("Error finding user: %v", err)
		return nil, status.Error(codes.Internal, "failed to find user")
	}

	// Generate new password
	newPassword := generateRandomPassword()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return nil, status.Error(codes.Internal, "failed to generate password")
	}

	// Update password
	result, err := s.usersCol.UpdateOne(
		queryCtx,
		bson.M{"_id": req.UserId},
		bson.M{
			"$set": bson.M{
				"password_hash": string(passwordHash),
				"updated_at":    primitive.NewDateTimeFromTime(time.Now()),
			},
		},
	)
	if err != nil {
		log.Printf("Error resetting password: %v", err)
		return nil, status.Error(codes.Internal, "failed to reset password")
	}

	if result.MatchedCount == 0 {
		return &pb.ResetPasswordResponse{
			Success: false,
			Message: "user not found",
		}, nil
	}

	log.Printf("Password reset successfully for user: %s", req.UserId)

	return &pb.ResetPasswordResponse{
		Success:     true,
		NewPassword: newPassword,
		Message:     "password reset successfully",
	}, nil
}

// ToggleUserStatus activates or deactivates a user account
func (s *AdminService) ToggleUserStatus(ctx context.Context, req *pb.ToggleUserStatusRequest) (*pb.ToggleUserStatusResponse, error) {
	if req == nil || req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Update user status
	result, err := s.usersCol.UpdateOne(
		queryCtx,
		bson.M{"_id": req.UserId},
		bson.M{
			"$set": bson.M{
				"is_active":  req.Activate,
				"updated_at": primitive.NewDateTimeFromTime(time.Now()),
			},
		},
	)
	if err != nil {
		log.Printf("Error toggling user status: %v", err)
		return nil, status.Error(codes.Internal, "failed to toggle user status")
	}

	if result.MatchedCount == 0 {
		return &pb.ToggleUserStatusResponse{
			Success: false,
			Message: "user not found",
		}, nil
	}

	action := "deactivated"
	if req.Activate {
		action = "activated"
	}

	log.Printf("User %s successfully: %s", action, req.UserId)

	return &pb.ToggleUserStatusResponse{
		Success: true,
		Message: fmt.Sprintf("user %s successfully", action),
	}, nil
}

// ============================================================================
// System Configuration
// ============================================================================

// SetEnrollmentPeriod sets the enrollment period dates
func (s *AdminService) SetEnrollmentPeriod(ctx context.Context, req *pb.SetEnrollmentPeriodRequest) (*pb.SetEnrollmentPeriodResponse, error) {
	if req == nil || req.StartDate == "" || req.EndDate == "" {
		return nil, status.Error(codes.InvalidArgument, "start_date and end_date are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Parse dates to validate format
	startTime, err := time.Parse(time.RFC3339, req.StartDate)
	if err != nil {
		return &pb.SetEnrollmentPeriodResponse{
			Success: false,
			Message: "invalid start_date format (use ISO 8601)",
		}, nil
	}

	endTime, err := time.Parse(time.RFC3339, req.EndDate)
	if err != nil {
		return &pb.SetEnrollmentPeriodResponse{
			Success: false,
			Message: "invalid end_date format (use ISO 8601)",
		}, nil
	}

	if endTime.Before(startTime) {
		return &pb.SetEnrollmentPeriodResponse{
			Success: false,
			Message: "end_date must be after start_date",
		}, nil
	}

	// Update enrollment_start
	_, err = s.updateSystemConfig(queryCtx, "enrollment_start", req.StartDate, "")
	if err != nil {
		log.Printf("Error setting enrollment_start: %v", err)
		return nil, status.Error(codes.Internal, "failed to set enrollment start date")
	}

	// Update enrollment_end
	_, err = s.updateSystemConfig(queryCtx, "enrollment_end", req.EndDate, "")
	if err != nil {
		log.Printf("Error setting enrollment_end: %v", err)
		return nil, status.Error(codes.Internal, "failed to set enrollment end date")
	}

	log.Printf("Enrollment period set: %s to %s", req.StartDate, req.EndDate)

	return &pb.SetEnrollmentPeriodResponse{
		Success: true,
		Message: "enrollment period set successfully",
	}, nil
}

// ToggleEnrollment enables or disables enrollment system-wide
func (s *AdminService) ToggleEnrollment(ctx context.Context, req *pb.ToggleEnrollmentRequest) (*pb.ToggleEnrollmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	value := "false"
	if req.Enable {
		value = "true"
	}

	_, err := s.updateSystemConfig(queryCtx, "enrollment_enabled", value, "")
	if err != nil {
		log.Printf("Error toggling enrollment: %v", err)
		return nil, status.Error(codes.Internal, "failed to toggle enrollment")
	}

	action := "disabled"
	if req.Enable {
		action = "enabled"
	}

	log.Printf("Enrollment %s system-wide", action)

	return &pb.ToggleEnrollmentResponse{
		Success:        true,
		EnrollmentOpe