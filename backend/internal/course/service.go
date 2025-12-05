package course

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/internal/pb/course"
	"stdiscm_p4/backend/internal/shared"
)

// CourseService implements the gRPC CourseService
type CourseService struct {
	pb.UnimplementedCourseServiceServer
	db               *mongo.Database
	coursesCol       *mongo.Collection
	prerequisitesCol *mongo.Collection
	enrollmentsCol   *mongo.Collection
	gradesCol        *mongo.Collection
}

// NewCourseService creates a new CourseService instance
func NewCourseService(db *mongo.Database) *CourseService {
	return &CourseService{
		db:               db,
		coursesCol:       db.Collection("courses"),
		prerequisitesCol: db.Collection("prerequisites"),
		enrollmentsCol:   db.Collection("enrollments"),
		gradesCol:        db.Collection("grades"),
	}
}

// ListCourses retrieves courses based on filters
func (s *CourseService) ListCourses(ctx context.Context, req *pb.ListCoursesRequest) (*pb.ListCoursesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Build filter query
	filter := bson.M{}

	if req.Filters != nil {
		// Filter by department (extract from course code)
		if req.Filters.Department != "" {
			filter["code"] = bson.M{
				"$regex": primitive.Regex{
					Pattern: "^" + strings.ToUpper(req.Filters.Department),
					Options: "i",
				},
			}
		}

		// Search query (course code or title)
		if req.Filters.SearchQuery != "" {
			searchRegex := primitive.Regex{
				Pattern: req.Filters.SearchQuery,
				Options: "i",
			}
			filter["$or"] = []bson.M{
				{"code": searchRegex},
				{"title": searchRegex},
			}
		}

		// Filter by open status
		if req.Filters.OpenOnly {
			filter["is_open"] = true
		}

		// Filter by semester
		if req.Filters.Semester != "" {
			filter["semester"] = req.Filters.Semester
		}
	}

	// Set query options using shared helper
	findOptions := shared.BuildFindOptions(100, "code", 1)

	// Execute query with timeout
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cursor, err := s.coursesCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying courses: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve courses")
	}
	defer cursor.Close(queryCtx)

	// Parse results
	var courses []*pb.Course
	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding course document: %v", err)
			continue
		}

		course, err := s.documentToCourse(queryCtx, doc)
		if err != nil {
			log.Printf("Error converting document to course: %v", err)
			continue
		}

		courses = append(courses, course)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, status.Error(codes.Internal, "error iterating courses")
	}

	// Get total count using shared helper
	totalCount, err := shared.CountDocumentsWithTimeout(ctx, s.coursesCol, filter, 5*time.Second)
	if err != nil {
		log.Printf("Error counting courses: %v", err)
		totalCount = int64(len(courses))
	}

	return &pb.ListCoursesResponse{
		Courses:    courses,
		TotalCount: int32(totalCount),
	}, nil
}

// GetCourse retrieves a single course by ID
func (s *CourseService) GetCourse(ctx context.Context, req *pb.GetCourseRequest) (*pb.GetCourseResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	var doc bson.M
	err := shared.FindOneWithTimeout(ctx, s.coursesCol, bson.M{"_id": req.CourseId}, &doc, 5*time.Second)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.GetCourseResponse{
				Success: false,
				Course:  nil,
				Message: fmt.Sprintf("course not found: %s", req.CourseId),
			}, nil
		}
		log.Printf("Error finding course %s: %v", req.CourseId, err)
		return nil, status.Error(codes.Internal, "failed to retrieve course")
	}

	course, err := s.documentToCourse(ctx, doc)
	if err != nil {
		log.Printf("Error converting document to course: %v", err)
		return nil, status.Error(codes.Internal, "failed to parse course data")
	}

	return &pb.GetCourseResponse{
		Success: true,
		Course:  course,
		Message: "course retrieved successfully",
	}, nil
}

// CheckPrerequisites verifies if a student has met prerequisites for a course
func (s *CourseService) CheckPrerequisites(ctx context.Context, req *pb.CheckPrerequisitesRequest) (*pb.CheckPrerequisitesResponse, error) {
	if req == nil || req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get prerequisites for the course
	cursor, err := s.prerequisitesCol.Find(queryCtx, bson.M{"course_id": req.CourseId})
	if err != nil {
		log.Printf("Error querying prerequisites: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve prerequisites")
	}
	defer cursor.Close(queryCtx)

	var prerequisiteIDs []string
	for cursor.Next(queryCtx) {
		var prereq shared.Prerequisite
		if err := cursor.Decode(&prereq); err != nil {
			log.Printf("Error decoding prerequisite: %v", err)
			continue
		}
		prerequisiteIDs = append(prerequisiteIDs, prereq.PrereqID)
	}

	// If no prerequisites, return success
	if len(prerequisiteIDs) == 0 {
		return &pb.CheckPrerequisitesResponse{
			AllMet:        true,
			Prerequisites: []*pb.PrerequisiteStatus{},
			Message:       "no prerequisites required",
		}, nil
	}

	// Check each prerequisite
	var prerequisiteStatuses []*pb.PrerequisiteStatus
	allMet := true

	for _, prereqID := range prerequisiteIDs {
		prereqStatus := s.checkSinglePrerequisite(queryCtx, req.StudentId, prereqID)
		prerequisiteStatuses = append(prerequisiteStatuses, prereqStatus)
		if !prereqStatus.Met {
			allMet = false
		}
	}

	message := "all prerequisites met"
	if !allMet {
		message = "some prerequisites not met"
	}

	return &pb.CheckPrerequisitesResponse{
		AllMet:        allMet,
		Prerequisites: prerequisiteStatuses,
		Message:       message,
	}, nil
}

// GetCourseAvailability checks if a course has available seats
func (s *CourseService) GetCourseAvailability(ctx context.Context, req *pb.GetCourseAvailabilityRequest) (*pb.GetCourseAvailabilityResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	var course shared.Course
	err := shared.FindOneWithTimeout(ctx, s.coursesCol, bson.M{"_id": req.CourseId}, &course, 5*time.Second)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.GetCourseAvailabilityResponse{
				Available:      false,
				Capacity:       0,
				Enrolled:       0,
				SeatsRemaining: 0,
				IsOpen:         false,
				Message:        fmt.Sprintf("course not found: %s", req.CourseId),
			}, nil
		}
		log.Printf("Error finding course availability: %v", err)
		return nil, status.Error(codes.Internal, "failed to check availability")
	}

	seatsRemaining := course.GetSeatsAvailable()
	available := course.IsAvailable()

	message := "course available"
	if !course.IsOpen {
		message = "course is closed"
	} else if seatsRemaining == 0 {
		message = "course is full"
	}

	return &pb.GetCourseAvailabilityResponse{
		Available:      available,
		Capacity:       course.Capacity,
		Enrolled:       course.Enrolled,
		SeatsRemaining: seatsRemaining,
		IsOpen:         course.IsOpen,
		Message:        message,
	}, nil
}

// ============================================================================
// Helper Functions (Private to service.go)
// ============================================================================

// documentToCourse converts a MongoDB document to a protobuf Course message
func (s *CourseService) documentToCourse(ctx context.Context, doc bson.M) (*pb.Course, error) {
	course := &pb.Course{}

	// Required fields using shared helpers
	id, err := shared.GetString(doc["_id"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid _id field")
	}
	course.Id = id

	code, err := shared.GetString(doc["code"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid code field")
	}
	course.Code = code

	title, err := shared.GetString(doc["title"])
	if err != nil {
		return nil, fmt.Errorf("missing or invalid title field")
	}
	course.Title = title

	// Optional fields with safe type assertions using shared helpers
	if description, err := shared.GetString(doc["description"]); err == nil {
		course.Description = description
	}

	if units, err := shared.GetInt32(doc["units"]); err == nil {
		course.Units = units
	}

	if schedule, err := shared.GetString(doc["schedule"]); err == nil {
		course.Schedule = schedule
	}

	if room, err := shared.GetString(doc["room"]); err == nil {
		course.Room = room
	}

	if capacity, err := shared.GetInt32(doc["capacity"]); err == nil {
		course.Capacity = capacity
	}

	if enrolled, err := shared.GetInt32(doc["enrolled"]); err == nil {
		course.Enrolled = enrolled
	}

	if facultyID, err := shared.GetString(doc["faculty_id"]); err == nil {
		course.FacultyId = facultyID
		// Get faculty name (optional)
		course.FacultyName = s.getFacultyName(ctx, facultyID)
	}

	if isOpen, err := shared.GetBool(doc["is_open"]); err == nil {
		course.IsOpen = isOpen
	}

	if semester, err := shared.GetString(doc["semester"]); err == nil {
		course.Semester = semester
	}

	// Timestamps using shared helper
	if createdAt, err := shared.GetTime(doc["created_at"]); err == nil {
		course.CreatedAt = timestamppb.New(createdAt)
	}

	if updatedAt, err := shared.GetTime(doc["updated_at"]); err == nil {
		course.UpdatedAt = timestamppb.New(updatedAt)
	}

	// Get prerequisites
	course.Prerequisites = s.getCoursePrerequisites(ctx, course.Id)

	return course, nil
}

// getFacultyName retrieves faculty name from users collection
func (s *CourseService) getFacultyName(ctx context.Context, facultyID string) string {
	var user shared.User

	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := s.db.Collection("users").FindOne(queryCtx, bson.M{"_id": facultyID}).Decode(&user)
	if err != nil {
		log.Printf("Warning: Could not fetch faculty name for %s: %v", facultyID, err)
		return ""
	}

	return user.Name
}

// getCoursePrerequisites retrieves prerequisite course IDs
func (s *CourseService) getCoursePrerequisites(ctx context.Context, courseID string) []string {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cursor, err := s.prerequisitesCol.Find(queryCtx, bson.M{"course_id": courseID})
	if err != nil {
		log.Printf("Warning: Could not fetch prerequisites for %s: %v", courseID, err)
		return []string{}
	}
	defer cursor.Close(queryCtx)

	var prerequisites []string
	for cursor.Next(queryCtx) {
		var prereq shared.Prerequisite
		if err := cursor.Decode(&prereq); err != nil {
			continue
		}
		prerequisites = append(prerequisites, prereq.PrereqID)
	}

	return prerequisites
}

// checkSinglePrerequisite checks if student has completed a specific prerequisite
func (s *CourseService) checkSinglePrerequisite(ctx context.Context, studentID, prereqCourseID string) *pb.PrerequisiteStatus {
	prereqStatus := &pb.PrerequisiteStatus{
		CourseId: prereqCourseID,
		Met:      false,
		Grade:    "",
	}

	// Get course code for display
	var course shared.Course
	if err := s.coursesCol.FindOne(ctx, bson.M{"_id": prereqCourseID}).Decode(&course); err == nil {
		prereqStatus.CourseCode = course.Code
	}

	// Find enrollment for this student and prerequisite course
	var enrollment shared.Enrollment
	err := s.enrollmentsCol.FindOne(ctx, bson.M{
		"student_id": studentID,
		"course_id":  prereqCourseID,
		"status":     shared.StatusCompleted,
	}).Decode(&enrollment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return prereqStatus // Not completed
		}
		log.Printf("Error checking enrollment for prerequisite: %v", err)
		return prereqStatus
	}

	// Check grade
	var grade shared.Grade
	err = s.gradesCol.FindOne(ctx, bson.M{
		"enrollment_id": enrollment.ID,
		"published":     true,
	}).Decode(&grade)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return prereqStatus // Grade not published yet
		}
		log.Printf("Error checking grade for prerequisite: %v", err)
		return prereqStatus
	}

	prereqStatus.Grade = grade.Grade

	// Check if grade is passing using shared helper
	if shared.IsPassingGrade(grade.Grade) {
		prereqStatus.Met = true
	}

	return prereqStatus
}
