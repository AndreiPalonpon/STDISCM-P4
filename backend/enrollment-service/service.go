package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/pb/enrollment"
)

// EnrollmentService implements the gRPC EnrollmentService
type EnrollmentService struct {
	pb.UnimplementedEnrollmentServiceServer
	db               *mongo.Database
	cartsCol         *mongo.Collection
	enrollmentsCol   *mongo.Collection
	coursesCol       *mongo.Collection
	prerequisitesCol *mongo.Collection
	gradesCol        *mongo.Collection
	auditLogsCol     *mongo.Collection
}

// NewEnrollmentService creates a new EnrollmentService instance
func NewEnrollmentService(db *mongo.Database) *EnrollmentService {
	return &EnrollmentService{
		db:               db,
		cartsCol:         db.Collection("carts"),
		enrollmentsCol:   db.Collection("enrollments"),
		coursesCol:       db.Collection("courses"),
		prerequisitesCol: db.Collection("prerequisites"),
		gradesCol:        db.Collection("grades"),
		auditLogsCol:     db.Collection("audit_logs"),
	}
}

// AddToCart adds a course to student's shopping cart
func (s *EnrollmentService) AddToCart(ctx context.Context, req *pb.AddToCartRequest) (*pb.AddToCartResponse, error) {
	if req == nil || req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if course exists and is open
	var course bson.M
	err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&course)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.AddToCartResponse{
				Success: false,
				Message: fmt.Sprintf("course not found: %s", req.CourseId),
				Cart:    nil,
			}, nil
		}
		log.Printf("Error finding course: %v", err)
		return nil, status.Error(codes.Internal, "failed to check course")
	}

	isOpen, _ := course["is_open"].(bool)
	if !isOpen {
		return &pb.AddToCartResponse{
			Success: false,
			Message: "course is not open for enrollment",
			Cart:    nil,
		}, nil
	}

	// Get or create cart
	var cartDoc bson.M
	err = s.cartsCol.FindOne(queryCtx, bson.M{"student_id": req.StudentId}).Decode(&cartDoc)

	if err != nil && err != mongo.ErrNoDocuments {
		log.Printf("Error finding cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve cart")
	}

	// Initialize cart if not exists
	var courseIDs []string
	if err == mongo.ErrNoDocuments {
		courseIDs = []string{}
	} else {
		if ids, ok := cartDoc["course_ids"].(primitive.A); ok {
			for _, id := range ids {
				if idStr, ok := id.(string); ok {
					courseIDs = append(courseIDs, idStr)
				}
			}
		}
	}

	// Check if course already in cart
	for _, id := range courseIDs {
		if id == req.CourseId {
			return &pb.AddToCartResponse{
				Success: false,
				Message: "course already in cart",
				Cart:    nil,
			}, nil
		}
	}

	// Check cart size limit (max 6 courses)
	if len(courseIDs) >= 6 {
		return &pb.AddToCartResponse{
			Success: false,
			Message: "cart is full (maximum 6 courses)",
			Cart:    nil,
		}, nil
	}

	// Check if student is already enrolled in this course
	enrollCount, err := s.enrollmentsCol.CountDocuments(queryCtx, bson.M{
		"student_id": req.StudentId,
		"course_id":  req.CourseId,
		"status":     "enrolled",
	})
	if err != nil {
		log.Printf("Error checking existing enrollment: %v", err)
		return nil, status.Error(codes.Internal, "failed to check enrollment status")
	}
	if enrollCount > 0 {
		return &pb.AddToCartResponse{
			Success: false,
			Message: "already enrolled in this course",
			Cart:    nil,
		}, nil
	}

	// Add course to cart
	courseIDs = append(courseIDs, req.CourseId)

	// Calculate total units
	totalUnits, err := s.calculateTotalUnits(queryCtx, courseIDs)
	if err != nil {
		log.Printf("Error calculating units: %v", err)
		return nil, status.Error(codes.Internal, "failed to calculate units")
	}

	// Check unit limit (max 18 units)
	if totalUnits > 18 {
		return &pb.AddToCartResponse{
			Success: false,
			Message: fmt.Sprintf("exceeds maximum units (18). Current total would be: %d", totalUnits),
			Cart:    nil,
		}, nil
	}

	// Update or insert cart
	update := bson.M{
		"$set": bson.M{
			"student_id": req.StudentId,
			"course_ids": courseIDs,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err = s.cartsCol.UpdateOne(queryCtx, bson.M{"student_id": req.StudentId}, update, opts)
	if err != nil {
		log.Printf("Error updating cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to update cart")
	}

	// Get updated cart with validation
	cart, err := s.buildCartResponse(queryCtx, req.StudentId, courseIDs)
	if err != nil {
		log.Printf("Error building cart response: %v", err)
		return nil, status.Error(codes.Internal, "failed to build cart")
	}

	return &pb.AddToCartResponse{
		Success: true,
		Message: "course added to cart successfully",
		Cart:    cart,
	}, nil
}

// RemoveFromCart removes a course from student's shopping cart
func (s *EnrollmentService) RemoveFromCart(ctx context.Context, req *pb.RemoveFromCartRequest) (*pb.RemoveFromCartResponse, error) {
	if req == nil || req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Remove course from cart
	result, err := s.cartsCol.UpdateOne(
		queryCtx,
		bson.M{"student_id": req.StudentId},
		bson.M{
			"$pull": bson.M{"course_ids": req.CourseId},
			"$set":  bson.M{"updated_at": primitive.NewDateTimeFromTime(time.Now())},
		},
	)

	if err != nil {
		log.Printf("Error removing from cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to remove course from cart")
	}

	if result.MatchedCount == 0 {
		return &pb.RemoveFromCartResponse{
			Success: false,
			Message: "cart not found",
			Cart:    nil,
		}, nil
	}

	// Get updated cart
	var cartDoc bson.M
	err = s.cartsCol.FindOne(queryCtx, bson.M{"student_id": req.StudentId}).Decode(&cartDoc)
	if err != nil {
		log.Printf("Error retrieving updated cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve cart")
	}

	var courseIDs []string
	if ids, ok := cartDoc["course_ids"].(primitive.A); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				courseIDs = append(courseIDs, idStr)
			}
		}
	}

	cart, err := s.buildCartResponse(queryCtx, req.StudentId, courseIDs)
	if err != nil {
		log.Printf("Error building cart response: %v", err)
		return nil, status.Error(codes.Internal, "failed to build cart")
	}

	return &pb.RemoveFromCartResponse{
		Success: true,
		Message: "course removed from cart successfully",
		Cart:    cart,
	}, nil
}

// GetCart retrieves student's shopping cart
func (s *EnrollmentService) GetCart(ctx context.Context, req *pb.GetCartRequest) (*pb.GetCartResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cartDoc bson.M
	err := s.cartsCol.FindOne(queryCtx, bson.M{"student_id": req.StudentId}).Decode(&cartDoc)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Return empty cart
			return &pb.GetCartResponse{
				Success: true,
				Cart: &pb.Cart{
					StudentId:            req.StudentId,
					Items:                []*pb.CartItem{},
					TotalUnits:           0,
					HasConflicts:         false,
					MissingPrerequisites: []string{},
					UpdatedAt:            timestamppb.Now(),
				},
				Message: "cart is empty",
			}, nil
		}
		log.Printf("Error finding cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve cart")
	}

	var courseIDs []string
	if ids, ok := cartDoc["course_ids"].(primitive.A); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				courseIDs = append(courseIDs, idStr)
			}
		}
	}

	cart, err := s.buildCartResponse(queryCtx, req.StudentId, courseIDs)
	if err != nil {
		log.Printf("Error building cart response: %v", err)
		return nil, status.Error(codes.Internal, "failed to build cart")
	}

	return &pb.GetCartResponse{
		Success: true,
		Cart:    cart,
		Message: "cart retrieved successfully",
	}, nil
}

// ClearCart removes all courses from student's cart
func (s *EnrollmentService) ClearCart(ctx context.Context, req *pb.ClearCartRequest) (*pb.ClearCartResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.cartsCol.DeleteOne(queryCtx, bson.M{"student_id": req.StudentId})
	if err != nil {
		log.Printf("Error clearing cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to clear cart")
	}

	return &pb.ClearCartResponse{
		Success: true,
		Message: "cart cleared successfully",
	}, nil
}

// CheckConflicts checks for schedule conflicts among courses
func (s *EnrollmentService) CheckConflicts(ctx context.Context, req *pb.CheckConflictsRequest) (*pb.CheckConflictsResponse, error) {
	if req == nil || req.StudentId == "" || len(req.CourseIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_ids are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conflicts, err := s.detectConflicts(queryCtx, req.StudentId, req.CourseIds)
	if err != nil {
		log.Printf("Error detecting conflicts: %v", err)
		return nil, status.Error(codes.Internal, "failed to check conflicts")
	}

	hasConflicts := len(conflicts) > 0
	message := "no conflicts detected"
	if hasConflicts {
		message = fmt.Sprintf("found %d conflict(s)", len(conflicts))
	}

	return &pb.CheckConflictsResponse{
		HasConflicts: hasConflicts,
		Conflicts:    conflicts,
		Message:      message,
	}, nil
}

// EnrollAll enrolls student in all courses in cart (transactional)
func (s *EnrollmentService) EnrollAll(ctx context.Context, req *pb.EnrollAllRequest) (*pb.EnrollAllResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get cart
	var cartDoc bson.M
	err := s.cartsCol.FindOne(queryCtx, bson.M{"student_id": req.StudentId}).Decode(&cartDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.EnrollAllResponse{
				Success:       false,
				Message:       "cart is empty",
				Enrollments:   []*pb.Enrollment{},
				FailedCourses: []string{},
			}, nil
		}
		log.Printf("Error finding cart: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve cart")
	}

	var courseIDs []string
	if ids, ok := cartDoc["course_ids"].(primitive.A); ok {
		for _, id := range ids {
			if idStr, ok := id.(string); ok {
				courseIDs = append(courseIDs, idStr)
			}
		}
	}

	if len(courseIDs) == 0 {
		return &pb.EnrollAllResponse{
			Success:       false,
			Message:       "cart is empty",
			Enrollments:   []*pb.Enrollment{},
			FailedCourses: []string{},
		}, nil
	}

	// Validate before enrollment
	conflicts, err := s.detectConflicts(queryCtx, req.StudentId, courseIDs)
	if err != nil {
		log.Printf("Error checking conflicts: %v", err)
		return nil, status.Error(codes.Internal, "failed to validate enrollment")
	}
	if len(conflicts) > 0 {
		return &pb.EnrollAllResponse{
			Success:       false,
			Message:       "cannot enroll: schedule conflicts detected",
			Enrollments:   []*pb.Enrollment{},
			FailedCourses: courseIDs,
		}, nil
	}

	// Check prerequisites for all courses
	for _, courseID := range courseIDs {
		prereqsMet, err := s.checkPrerequisitesMet(queryCtx, req.StudentId, courseID)
		if err != nil {
			log.Printf("Error checking prerequisites: %v", err)
			return nil, status.Error(codes.Internal, "failed to validate prerequisites")
		}
		if !prereqsMet {
			return &pb.EnrollAllResponse{
				Success:       false,
				Message:       fmt.Sprintf("prerequisites not met for course: %s", courseID),
				Enrollments:   []*pb.Enrollment{},
				FailedCourses: []string{courseID},
			}, nil
		}
	}

	// Start MongoDB session for transaction
	session, err := s.db.Client().StartSession()
	if err != nil {
		log.Printf("Error starting session: %v", err)
		return nil, status.Error(codes.Internal, "failed to start transaction")
	}
	defer session.EndSession(queryCtx)

	// Execute transaction
	var enrollments []*pb.Enrollment
	var failedCourses []string

	_, err = session.WithTransaction(queryCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		for _, courseID := range courseIDs {
			// Check course availability and capacity
			var course bson.M
			err := s.coursesCol.FindOne(sessCtx, bson.M{"_id": courseID}).Decode(&course)
			if err != nil {
				failedCourses = append(failedCourses, courseID)
				return nil, fmt.Errorf("course not found: %s", courseID)
			}

			capacity, _ := getInt32(course["capacity"])
			enrolled, _ := getInt32(course["enrolled"])
			isOpen, _ := course["is_open"].(bool)

			if !isOpen {
				failedCourses = append(failedCourses, courseID)
				return nil, fmt.Errorf("course is closed: %s", courseID)
			}

			if enrolled >= capacity {
				failedCourses = append(failedCourses, courseID)
				return nil, fmt.Errorf("course is full: %s", courseID)
			}

			// Create enrollment
			enrollmentID := generateEnrollmentID()
			scheduleInfo := extractScheduleInfo(course)

			enrollmentDoc := bson.M{
				"_id":           enrollmentID,
				"student_id":    req.StudentId,
				"course_id":     courseID,
				"status":        "enrolled",
				"enrolled_at":   primitive.NewDateTimeFromTime(time.Now()),
				"schedule_info": scheduleInfo,
			}

			_, err = s.enrollmentsCol.InsertOne(sessCtx, enrollmentDoc)
			if err != nil {
				failedCourses = append(failedCourses, courseID)
				return nil, fmt.Errorf("failed to create enrollment: %v", err)
			}

			// Increment enrolled count
			_, err = s.coursesCol.UpdateOne(
				sessCtx,
				bson.M{"_id": courseID},
				bson.M{"$inc": bson.M{"enrolled": 1}},
			)
			if err != nil {
				return nil, fmt.Errorf("failed to update course capacity: %v", err)
			}

			// Build enrollment response
			enrollment := &pb.Enrollment{
				Id:         enrollmentID,
				StudentId:  req.StudentId,
				CourseId:   courseID,
				Status:     "enrolled",
				EnrolledAt: timestamppb.Now(),
			}

			if code, ok := course["code"].(string); ok {
				enrollment.CourseCode = code
			}
			if title, ok := course["title"].(string); ok {
				enrollment.CourseTitle = title
			}
			if units, ok := getInt32(course["units"]); ok == nil {
				enrollment.Units = units
			}
			if schedInfo := extractScheduleInfoProto(course); schedInfo != nil {
				enrollment.ScheduleInfo = schedInfo
			}

			enrollments = append(enrollments, enrollment)
		}

		// Clear cart after successful enrollment
		_, err := s.cartsCol.DeleteOne(sessCtx, bson.M{"student_id": req.StudentId})
		if err != nil {
			log.Printf("Warning: Failed to clear cart: %v", err)
		}

		// Log audit event
		s.logAuditEvent(sessCtx, req.StudentId, "enroll", fmt.Sprintf("enrolled in %d courses", len(courseIDs)))

		return nil, nil
	})

	if err != nil {
		log.Printf("Transaction failed: %v", err)
		return &pb.EnrollAllResponse{
			Success:       false,
			Message:       fmt.Sprintf("enrollment failed: %v", err),
			Enrollments:   []*pb.Enrollment{},
			FailedCourses: failedCourses,
		}, nil
	}

	return &pb.EnrollAllResponse{
		Success:       true,
		Message:       fmt.Sprintf("successfully enrolled in %d course(s)", len(enrollments)),
		Enrollments:   enrollments,
		FailedCourses: []string{},
	}, nil
}

// DropCourse drops a student from an enrolled course
func (s *EnrollmentService) DropCourse(ctx context.Context, req *pb.DropCourseRequest) (*pb.DropCourseResponse, error) {
	if req == nil || req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Find enrollment
	var enrollment bson.M
	err := s.enrollmentsCol.FindOne(queryCtx, bson.M{
		"student_id": req.StudentId,
		"course_id":  req.CourseId,
		"status":     "enrolled",
	}).Decode(&enrollment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.DropCourseResponse{
				Success: false,
				Message: "enrollment not found",
			}, nil
		}
		log.Printf("Error finding enrollment: %v", err)
		return nil, status.Error(codes.Internal, "failed to find enrollment")
	}

	// Start transaction for drop
	session, err := s.db.Client().StartSession()
	if err != nil {
		log.Printf("Error starting session: %v", err)
		return nil, status.Error(codes.Internal, "failed to start transaction")
	}
	defer session.EndSession(queryCtx)

	_, err = session.WithTransaction(queryCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Update enrollment status
		_, err := s.enrollmentsCol.UpdateOne(
			sessCtx,
			bson.M{"_id": enrollment["_id"]},
			bson.M{
				"$set": bson.M{
					"status":     "dropped",
					"dropped_at": primitive.NewDateTimeFromTime(time.Now()),
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update enrollment: %v", err)
		}

		// Decrement enrolled count
		_, err = s.coursesCol.UpdateOne(
			sessCtx,
			bson.M{"_id": req.CourseId},
			bson.M{"$inc": bson.M{"enrolled": -1}},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update course capacity: %v", err)
		}

		// Log audit event
		s.logAuditEvent(sessCtx, req.StudentId, "drop", req.CourseId)

		return nil, nil
	})

	if err != nil {
		log.Printf("Transaction failed: %v", err)
		return &pb.DropCourseResponse{
			Success: false,
			Message: fmt.Sprintf("failed to drop course: %v", err),
		}, nil
	}

	return &pb.DropCourseResponse{
		Success: true,
		Message: "course dropped successfully",
	}, nil
}

// GetStudentEnrollments retrieves all enrollments for a student
func (s *EnrollmentService) GetStudentEnrollments(ctx context.Context, req *pb.GetStudentEnrollmentsRequest) (*pb.GetStudentEnrollmentsResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build filter
	filter := bson.M{"student_id": req.StudentId}
	if req.Semester != "" {
		filter["semester"] = req.Semester
	}
	if req.Status != "" {
		filter["status"] = req.Status
	} else {
		filter["status"] = "enrolled" // Default to enrolled only
	}

	cursor, err := s.enrollmentsCol.Find(queryCtx, filter)
	if err != nil {
		log.Printf("Error querying enrollments: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve enrollments")
	}
	defer cursor.Close(queryCtx)

	var enrollments []*pb.Enrollment
	totalUnits := int32(0)

	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding enrollment: %v", err)
			continue
		}

		enrollment, err := s.buildEnrollmentFromDoc(queryCtx, doc)
		if err != nil {
			log.Printf("Error building enrollment: %v", err)
			continue
		}

		enrollments = append(enrollments, enrollment)
		totalUnits += enrollment.Units
	}

	return &pb.GetStudentEnrollmentsResponse{
		Enrollments: enrollments,
		TotalUnits:  totalUnits,
	}, nil
}

// ============================================================================
// Helper Functions (Private to service.go)
// ============================================================================

// calculateTotalUnits calculates total units for given course IDs
func (s *EnrollmentService) calculateTotalUnits(ctx context.Context, courseIDs []string) (int32, error) {
	if len(courseIDs) == 0 {
		return 0, nil
	}

	cursor, err := s.coursesCol.Find(ctx, bson.M{"_id": bson.M{"$in": courseIDs}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	totalUnits := int32(0)
	for cursor.Next(ctx) {
		var course bson.M
		if err := cursor.Decode(&course); err != nil {
			continue
		}
		if units, ok := getInt32(course["units"]); ok == nil {
			totalUnits += units
		}
	}

	return totalUnits, nil
}

// buildCartResponse builds a complete cart response with validation
func (s *EnrollmentService) buildCartResponse(ctx context.Context, studentID string, courseIDs []string) (*pb.Cart, error) {
	cart := &pb.Cart{
		StudentId:            studentID,
		Items:                []*pb.CartItem{},
		TotalUnits:           0,
		HasConflicts:         false,
		MissingPrerequisites: []string{},
		UpdatedAt:            timestamppb.Now(),
	}

	if len(courseIDs) == 0 {
		return cart, nil
	}

	// Get course details
	cursor, err := s.coursesCol.Find(ctx, bson.M{"_id": bson.M{"$in": courseIDs}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var course bson.M
		if err := cursor.Decode(&course); err != nil {
			continue
		}

		item := &pb.CartItem{}
		if id, ok := course["_id"].(string); ok {
			item.CourseId = id
		}
		if code, ok := course["code"].(string); ok {
			item.CourseCode = code
		}
		if title, ok := course["title"].(string); ok {
			item.CourseTitle = title
		}
		if units, ok := getInt32(course["units"]); ok == nil {
			item.Units = units
			cart.TotalUnits += units
		}
		if schedInfo := extractScheduleInfoProto(course); schedInfo != nil {
			item.ScheduleInfo = schedInfo
		}

		cart.Items = append(cart.Items, item)
	}

	// Check for conflicts
	conflicts, err := s.detectConflicts(ctx, studentID, courseIDs)
	if err == nil && len(conflicts) > 0 {
		cart.HasConflicts = true
	}

	// Check prerequisites
	for _, courseID := range courseIDs {
		prereqsMet, _ := s.checkPrerequisitesMet(ctx, studentID, courseID)
		if !prereqsMet {
			cart.MissingPrerequisites = append(cart.MissingPrerequisites, courseID)
		}
	}

	return cart, nil
}

type courseSchedule struct {
	ID        string
	Code      string
	Days      []string
	StartTime string
	EndTime   string
}

// detectConflicts checks for schedule conflicts and duplicates
func (s *EnrollmentService) detectConflicts(ctx context.Context, studentID string, courseIDs []string) ([]*pb.Conflict, error) {
	var conflicts []*pb.Conflict

	// Check for duplicates
	seen := make(map[string]bool)
	for _, id := range courseIDs {
		if seen[id] {
			conflicts = append(conflicts, &pb.Conflict{
				Course1Id:    id,
				Course2Id:    id,
				ConflictType: "duplicate",
				Details:      "course appears multiple times",
			})
		}
		seen[id] = true
	}

	// Get course schedules
	cursor, err := s.coursesCol.Find(ctx, bson.M{"_id": bson.M{"$in": courseIDs}})
	if err != nil {
		return conflicts, err
	}
	defer cursor.Close(ctx)

	var schedules []courseSchedule
	for cursor.Next(ctx) {
		var course bson.M
		if err := cursor.Decode(&course); err != nil {
			continue
		}

		sched := courseSchedule{}
		if id, ok := course["_id"].(string); ok {
			sched.ID = id
		}
		if code, ok := course["code"].(string); ok {
			sched.Code = code
		}

		if schedule, ok := course["schedule"].(string); ok {
			days, start, end := parseSchedule(schedule)
			sched.Days = days
			sched.StartTime = start
			sched.EndTime = end
		}

		schedules = append(schedules, sched)
	}

	// Check for schedule conflicts
	for i := 0; i < len(schedules); i++ {
		for j := i + 1; j < len(schedules); j++ {
			if hasScheduleConflict(schedules[i], schedules[j]) {
				conflicts = append(conflicts, &pb.Conflict{
					Course1Id:    schedules[i].ID,
					Course1Code:  schedules[i].Code,
					Course2Id:    schedules[j].ID,
					Course2Code:  schedules[j].Code,
					ConflictType: "schedule",
					Details:      "time slots overlap",
				})
			}
		}
	}

	return conflicts, nil
}

// checkPrerequisitesMet checks if student has met all prerequisites for a course
func (s *EnrollmentService) checkPrerequisitesMet(ctx context.Context, studentID, courseID string) (bool, error) {
	// Get prerequisites for the course
	cursor, err := s.prerequisitesCol.Find(ctx, bson.M{"course_id": courseID})
	if err != nil {
		return false, err
	}
	defer cursor.Close(ctx)

	var prerequisiteIDs []string
	for cursor.Next(ctx) {
		var prereq struct {
			PrereqID string `bson:"prereq_id"`
		}
		if err := cursor.Decode(&prereq); err != nil {
			continue
		}
		prerequisiteIDs = append(prerequisiteIDs, prereq.PrereqID)
	}

	// If no prerequisites, return true
	if len(prerequisiteIDs) == 0 {
		return true, nil
	}

	// Check each prerequisite
	for _, prereqID := range prerequisiteIDs {
		// Find completed enrollment
		var enrollment struct {
			ID     string `bson:"_id"`
			Status string `bson:"status"`
		}

		err := s.enrollmentsCol.FindOne(ctx, bson.M{
			"student_id": studentID,
			"course_id":  prereqID,
			"status":     "completed",
		}).Decode(&enrollment)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				return false, nil // Prerequisite not completed
			}
			return false, err
		}

		// Check if grade is passing
		var grade struct {
			Grade     string `bson:"grade"`
			Published bool   `bson:"published"`
		}

		err = s.gradesCol.FindOne(ctx, bson.M{
			"enrollment_id": enrollment.ID,
			"published":     true,
		}).Decode(&grade)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				return false, nil // Grade not published yet
			}
			return false, err
		}

		// Check if grade is passing (not F or W)
		if grade.Grade == "F" || grade.Grade == "W" {
			return false, nil
		}
	}

	return true, nil
}

// buildEnrollmentFromDoc converts MongoDB document to Enrollment proto
func (s *EnrollmentService) buildEnrollmentFromDoc(ctx context.Context, doc bson.M) (*pb.Enrollment, error) {
	enrollment := &pb.Enrollment{}

	if id, ok := doc["_id"].(string); ok {
		enrollment.Id = id
	}
	if studentID, ok := doc["student_id"].(string); ok {
		enrollment.StudentId = studentID
	}
	if courseID, ok := doc["course_id"].(string); ok {
		enrollment.CourseId = courseID

		// Get course details
		var course bson.M
		err := s.coursesCol.FindOne(ctx, bson.M{"_id": courseID}).Decode(&course)
		if err == nil {
			if code, ok := course["code"].(string); ok {
				enrollment.CourseCode = code
			}
			if title, ok := course["title"].(string); ok {
				enrollment.CourseTitle = title
			}
			if units, ok := getInt32(course["units"]); ok == nil {
				enrollment.Units = units
			}
		}
	}
	if status, ok := doc["status"].(string); ok {
		enrollment.Status = status
	}
	if enrolledAt, ok := doc["enrolled_at"].(primitive.DateTime); ok {
		enrollment.EnrolledAt = timestamppb.New(enrolledAt.Time())
	}
	if droppedAt, ok := doc["dropped_at"].(primitive.DateTime); ok {
		enrollment.DroppedAt = timestamppb.New(droppedAt.Time())
	}

	// Extract schedule info
	if schedInfo, ok := doc["schedule_info"].(bson.M); ok {
		enrollment.ScheduleInfo = &pb.ScheduleInfo{}
		if days, ok := schedInfo["days"].(primitive.A); ok {
			for _, day := range days {
				if dayStr, ok := day.(string); ok {
					enrollment.ScheduleInfo.Days = append(enrollment.ScheduleInfo.Days, dayStr)
				}
			}
		}
		if startTime, ok := schedInfo["start_time"].(string); ok {
			enrollment.ScheduleInfo.StartTime = startTime
		}
		if endTime, ok := schedInfo["end_time"].(string); ok {
			enrollment.ScheduleInfo.EndTime = endTime
		}
	}

	return enrollment, nil
}

// extractScheduleInfo extracts schedule info from course document as bson.M
func extractScheduleInfo(course bson.M) bson.M {
	schedInfo := bson.M{}

	if schedule, ok := course["schedule"].(string); ok {
		days, startTime, endTime := parseSchedule(schedule)
		schedInfo["days"] = days
		schedInfo["start_time"] = startTime
		schedInfo["end_time"] = endTime
	}

	return schedInfo
}

// extractScheduleInfoProto extracts schedule info from course document as proto
func extractScheduleInfoProto(course bson.M) *pb.ScheduleInfo {
	if schedule, ok := course["schedule"].(string); ok {
		days, startTime, endTime := parseSchedule(schedule)
		return &pb.ScheduleInfo{
			Days:      days,
			StartTime: startTime,
			EndTime:   endTime,
		}
	}
	return nil
}

// parseSchedule parses schedule string (e.g., "MWF 9:00-10:00")
func parseSchedule(schedule string) ([]string, string, string) {
	parts := strings.Fields(schedule)
	if len(parts) < 2 {
		return []string{}, "", ""
	}

	// Parse days (e.g., "MWF" -> ["M", "W", "F"])
	dayStr := parts[0]
	var days []string

	i := 0
	for i < len(dayStr) {
		if i+1 < len(dayStr) && dayStr[i:i+2] == "TH" {
			days = append(days, "TH")
			i += 2
		} else {
			days = append(days, string(dayStr[i]))
			i++
		}
	}

	// Parse time range (e.g., "9:00-10:00")
	timeRange := parts[1]
	timeParts := strings.Split(timeRange, "-")
	startTime := ""
	endTime := ""

	if len(timeParts) == 2 {
		startTime = timeParts[0]
		endTime = timeParts[1]
	}

	return days, startTime, endTime
}

// hasScheduleConflict checks if two course schedules conflict
func hasScheduleConflict(course1 courseSchedule, course2 courseSchedule) bool {
	// Check if they share any days
	sharedDays := false
	for _, day1 := range course1.Days {
		for _, day2 := range course2.Days {
			if day1 == day2 {
				sharedDays = true
				break
			}
		}
		if sharedDays {
			break
		}
	}

	if !sharedDays {
		return false
	}

	// Check time overlap
	return timeRangesOverlap(course1.StartTime, course1.EndTime, course2.StartTime, course2.EndTime)
}

// timeRangesOverlap checks if two time ranges overlap
func timeRangesOverlap(start1, end1, start2, end2 string) bool {
	// Convert time strings to minutes since midnight for comparison
	start1Min := timeToMinutes(start1)
	end1Min := timeToMinutes(end1)
	start2Min := timeToMinutes(start2)
	end2Min := timeToMinutes(end2)

	// Check overlap: (start1 < end2) AND (start2 < end1)
	return start1Min < end2Min && start2Min < end1Min
}

// timeToMinutes converts time string (HH:MM) to minutes since midnight
func timeToMinutes(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0
	}

	hours := 0
	minutes := 0
	fmt.Sscanf(parts[0], "%d", &hours)
	fmt.Sscanf(parts[1], "%d", &minutes)

	return hours*60 + minutes
}

// generateEnrollmentID generates a unique enrollment ID
func generateEnrollmentID() string {
	return fmt.Sprintf("ENR%d%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
}

// getInt32 safely extracts int32 from interface{}
func getInt32(val interface{}) (int32, error) {
	switch v := val.(type) {
	case int32:
		return v, nil
	case int64:
		return int32(v), nil
	case int:
		return int32(v), nil
	case float64:
		return int32(v), nil
	default:
		return 0, fmt.Errorf("cannot convert to int32")
	}
}

// logAuditEvent logs an audit event to the database
func (s *EnrollmentService) logAuditEvent(ctx context.Context, userID, action, resource string) {
	auditLog := bson.M{
		"timestamp": primitive.NewDateTimeFromTime(time.Now()),
		"user_id":   userID,
		"action":    action,
		"resource":  resource,
	}

	_, err := s.auditLogsCol.InsertOne(ctx, auditLog)
	if err != nil {
		log.Printf("Warning: Failed to log audit event: %v", err)
	}
}
