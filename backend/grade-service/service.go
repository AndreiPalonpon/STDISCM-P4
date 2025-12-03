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

	pb "stdiscm_p4/backend/pb/grade"
)

// GradeService implements the gRPC GradeService
type GradeService struct {
	pb.UnimplementedGradeServiceServer
	db               *mongo.Database
	gradesCol        *mongo.Collection
	enrollmentsCol   *mongo.Collection
	coursesCol       *mongo.Collection
	usersCol         *mongo.Collection
	prerequisitesCol *mongo.Collection
}

// NewGradeService creates a new GradeService instance
func NewGradeService(db *mongo.Database) *GradeService {
	return &GradeService{
		db:               db,
		gradesCol:        db.Collection("grades"),
		enrollmentsCol:   db.Collection("enrollments"),
		coursesCol:       db.Collection("courses"),
		usersCol:         db.Collection("users"),
		prerequisitesCol: db.Collection("prerequisites"),
	}
}

// GetStudentGrades retrieves all grades for a student
func (s *GradeService) GetStudentGrades(ctx context.Context, req *pb.GetStudentGradesRequest) (*pb.GetStudentGradesResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Verify student exists
	var student struct {
		ID   string `bson:"_id"`
		Role string `bson:"role"`
	}
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.StudentId}).Decode(&student)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.GetStudentGradesResponse{
				Grades:  []*pb.Grade{},
				GpaInfo: &pb.GPACalculation{},
			}, nil
		}
		log.Printf("Error finding student %s: %v", req.StudentId, err)
		return nil, status.Error(codes.Internal, "failed to retrieve student information")
	}

	if student.Role != "student" {
		return nil, status.Error(codes.PermissionDenied, "user is not a student")
	}

	// Get grades for this student
	filter := bson.M{"student_id": req.StudentId}
	if req.Semester != "" {
		filter["semester"] = req.Semester
	}

	findOptions := options.Find().
		SetSort(bson.D{{Key: "semester", Value: -1}, {Key: "course_code", Value: 1}}).
		SetLimit(100)

	cursor, err := s.gradesCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying grades: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve grades")
	}
	defer cursor.Close(queryCtx)

	// Parse results
	var grades []*pb.Grade
	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding grade document: %v", err)
			continue
		}

		grade, err := s.documentToGrade(queryCtx, doc)
		if err != nil {
			log.Printf("Error converting document to grade: %v", err)
			continue
		}

		grades = append(grades, grade)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, status.Error(codes.Internal, "error iterating grades")
	}

	// Calculate GPA
	gpaInfo, err := s.calculateStudentGPA(queryCtx, req.StudentId, req.Semester)
	if err != nil {
		log.Printf("Error calculating GPA: %v", err)
		gpaInfo = &pb.GPACalculation{}
	}

	return &pb.GetStudentGradesResponse{
		Grades:  grades,
		GpaInfo: gpaInfo,
	}, nil
}

// CalculateGPA calculates GPA for a student
func (s *GradeService) CalculateGPA(ctx context.Context, req *pb.CalculateGPARequest) (*pb.CalculateGPAResponse, error) {
	if req == nil || req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Verify student exists
	var student struct {
		ID   string `bson:"_id"`
		Role string `bson:"role"`
	}
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.StudentId}).Decode(&student)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.CalculateGPAResponse{
				Success: false,
				GpaInfo: &pb.GPACalculation{},
				Message: fmt.Sprintf("student not found: %s", req.StudentId),
			}, nil
		}
		log.Printf("Error finding student %s: %v", req.StudentId, err)
		return nil, status.Error(codes.Internal, "failed to retrieve student information")
	}

	if student.Role != "student" {
		return nil, status.Error(codes.PermissionDenied, "user is not a student")
	}

	// Calculate GPA
	gpaInfo, err := s.calculateStudentGPA(queryCtx, req.StudentId, req.Semester)
	if err != nil {
		log.Printf("Error calculating GPA for student %s: %v", req.StudentId, err)
		return nil, status.Error(codes.Internal, "failed to calculate GPA")
	}

	return &pb.CalculateGPAResponse{
		Success: true,
		GpaInfo: gpaInfo,
		Message: "GPA calculated successfully",
	}, nil
}

// GetClassRoster retrieves all students enrolled in a course
func (s *GradeService) GetClassRoster(ctx context.Context, req *pb.GetClassRosterRequest) (*pb.GetClassRosterResponse, error) {
	if req == nil || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id is required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get course information
	var course struct {
		ID    string `bson:"_id"`
		Code  string `bson:"code"`
		Title string `bson:"title"`
	}
	err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&course)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.GetClassRosterResponse{
				CourseId:      req.CourseId,
				CourseCode:    "",
				CourseTitle:   "",
				Students:      []*pb.StudentRosterEntry{},
				TotalStudents: 0,
			}, nil
		}
		log.Printf("Error finding course %s: %v", req.CourseId, err)
		return nil, status.Error(codes.Internal, "failed to retrieve course information")
	}

	// Get all enrolled students for this course
	filter := bson.M{
		"course_id": req.CourseId,
		"status":    "enrolled",
	}

	findOptions := options.Find().
		SetSort(bson.D{{Key: "student_id", Value: 1}})

	cursor, err := s.enrollmentsCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying enrollments: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve enrollments")
	}
	defer cursor.Close(queryCtx)

	// Parse enrollments and build roster
	var students []*pb.StudentRosterEntry
	for cursor.Next(queryCtx) {
		var enrollment struct {
			ID        string `bson:"_id"`
			StudentID string `bson:"student_id"`
		}
		if err := cursor.Decode(&enrollment); err != nil {
			log.Printf("Error decoding enrollment: %v", err)
			continue
		}

		// Get student details
		studentEntry, err := s.getStudentRosterEntry(queryCtx, enrollment.StudentID, enrollment.ID)
		if err != nil {
			log.Printf("Error getting student entry for %s: %v", enrollment.StudentID, err)
			continue
		}

		students = append(students, studentEntry)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, status.Error(codes.Internal, "error iterating enrollments")
	}

	return &pb.GetClassRosterResponse{
		CourseId:      req.CourseId,
		CourseCode:    course.Code,
		CourseTitle:   course.Title,
		Students:      students,
		TotalStudents: int32(len(students)),
	}, nil
}

// UploadGrades handles streaming of grade entries
func (s *GradeService) UploadGrades(stream pb.GradeService_UploadGradesServer) error {
	log.Println("[GradeService] UploadGrades stream started")

	var (
		totalProcessed   int32
		successful       int32
		failed           int32
		errors           []string
		courseID         string
		facultyID        string
		receivedMetadata = false
	)

	// Process stream messages
	for {
		req, err := stream.Recv()
		if err != nil {
			// Stream ended
			break
		}

		// First message should be UploadGradesRequest (with course_id and faculty_id)
		if !receivedMetadata {
			if req.GetMetadata().GetCourseId() == "" || req.GetMetadata().GetFacultyId() == "" {
				return status.Error(codes.InvalidArgument, "first message must contain course_id and faculty_id")
			}

			courseID = req.GetMetadata().GetCourseId()
			facultyID = req.GetMetadata().GetFacultyId()

			// Validate faculty can upload grades for this course
			if err := s.validateFacultyForCourse(stream.Context(), courseID, facultyID); err != nil {
				return status.Errorf(codes.PermissionDenied, "faculty validation failed: %v", err)
			}

			receivedMetadata = true
			continue // Skip to next message for grade entries
		}

		// Process grade entry messages
		entry := req.GetEntry()
		if entry == nil {
			failed++
			errors = append(errors, "nil grade entry received")
			continue
		}

		totalProcessed++

		// Upload the grade
		if err := s.uploadSingleGrade(stream.Context(), courseID, facultyID, entry); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("student %s: %v", entry.StudentId, err))
			log.Printf("[GradeService] Failed to upload grade for student %s: %v", entry.StudentId, err)
		} else {
			successful++
		}

		// Check if this is the last entry
		if req.IsLast {
			break
		}
	}

	// Check if we received metadata
	if !receivedMetadata {
		return status.Error(codes.InvalidArgument, "no metadata received with course_id and faculty_id")
	}

	// Send final response
	response := &pb.UploadGradesResponse{
		Success:        successful > 0 || totalProcessed == 0,
		TotalProcessed: totalProcessed,
		Successful:     successful,
		Failed:         failed,
		Errors:         errors,
		Message:        fmt.Sprintf("Processed %d grades, %d successful, %d failed", totalProcessed, successful, failed),
	}

	return stream.SendAndClose(response)
}

// PublishGrades makes grades visible to students
func (s *GradeService) PublishGrades(ctx context.Context, req *pb.PublishGradesRequest) (*pb.PublishGradesResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id and faculty_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Validate faculty can publish grades for this course
	if err := s.validateFacultyForCourse(queryCtx, req.CourseId, req.FacultyId); err != nil {
		return &pb.PublishGradesResponse{
			Success:         false,
			GradesPublished: 0,
			Message:         fmt.Sprintf("faculty validation failed: %v", err),
		}, nil
	}

	// Find all grades for this course that aren't published yet
	filter := bson.M{
		"course_id": req.CourseId,
		"published": false,
	}

	update := bson.M{
		"$set": bson.M{
			"published":        true,
			"published_at":     time.Now(),
			"last_modified_by": req.FacultyId,
			"last_modified_at": time.Now(),
		},
	}

	result, err := s.gradesCol.UpdateMany(queryCtx, filter, update)
	if err != nil {
		log.Printf("Error publishing grades for course %s: %v", req.CourseId, err)
		return nil, status.Error(codes.Internal, "failed to publish grades")
	}

	message := "no grades to publish"
	if result.ModifiedCount > 0 {
		message = fmt.Sprintf("published %d grades", result.ModifiedCount)
	}

	return &pb.PublishGradesResponse{
		Success:         true,
		GradesPublished: int32(result.ModifiedCount),
		Message:         message,
	}, nil
}

// GetCourseGrades retrieves all grades for a course (faculty only)
func (s *GradeService) GetCourseGrades(ctx context.Context, req *pb.GetCourseGradesRequest) (*pb.GetCourseGradesResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "course_id and faculty_id are required")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Validate faculty can view grades for this course
	if err := s.validateFacultyForCourse(queryCtx, req.CourseId, req.FacultyId); err != nil {
		return &pb.GetCourseGradesResponse{
			Grades:       []*pb.Grade{},
			TotalGrades:  0,
			AllPublished: false,
		}, nil
	}

	// Get all grades for this course
	filter := bson.M{"course_id": req.CourseId}
	findOptions := options.Find().
		SetSort(bson.D{{Key: "student_id", Value: 1}})

	cursor, err := s.gradesCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying course grades: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve grades")
	}
	defer cursor.Close(queryCtx)

	// Parse results
	var grades []*pb.Grade
	allPublished := true

	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Error decoding grade document: %v", err)
			continue
		}

		grade, err := s.documentToGrade(queryCtx, doc)
		if err != nil {
			log.Printf("Error converting document to grade: %v", err)
			continue
		}

		grades = append(grades, grade)

		// Check if this grade is published
		if published, ok := doc["published"].(bool); ok && !published {
			allPublished = false
		}
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, status.Error(codes.Internal, "error iterating grades")
	}

	// Get total count
	totalCount, err := s.gradesCol.CountDocuments(queryCtx, filter)
	if err != nil {
		log.Printf("Error counting grades: %v", err)
		totalCount = int64(len(grades))
	}

	return &pb.GetCourseGradesResponse{
		Grades:       grades,
		TotalGrades:  int32(totalCount),
		AllPublished: allPublished && len(grades) > 0,
	}, nil
}

// ============================================================================
// Helper Functions (Private to service.go)
// ============================================================================

// documentToGrade converts a MongoDB document to a protobuf Grade message
func (s *GradeService) documentToGrade(ctx context.Context, doc bson.M) (*pb.Grade, error) {
	grade := &pb.Grade{}

	// Required fields
	if enrollmentID, ok := doc["enrollment_id"].(string); ok {
		grade.EnrollmentId = enrollmentID
	} else {
		return nil, fmt.Errorf("missing or invalid enrollment_id field")
	}

	if studentID, ok := doc["student_id"].(string); ok {
		grade.StudentId = studentID
	} else {
		return nil, fmt.Errorf("missing or invalid student_id field")
	}

	if studentName, ok := doc["student_name"].(string); ok {
		grade.StudentName = studentName
	}

	if courseID, ok := doc["course_id"].(string); ok {
		grade.CourseId = courseID
	}

	if courseCode, ok := doc["course_code"].(string); ok {
		grade.CourseCode = courseCode
	}

	if courseTitle, ok := doc["course_title"].(string); ok {
		grade.CourseTitle = courseTitle
	}

	if units, ok := doc["units"].(int32); ok {
		grade.Units = units
	} else if units, ok := doc["units"].(int64); ok {
		grade.Units = int32(units)
	}

	if gradeStr, ok := doc["grade"].(string); ok {
		grade.Grade = strings.ToUpper(gradeStr)
	} else {
		return nil, fmt.Errorf("missing or invalid grade field")
	}

	if semester, ok := doc["semester"].(string); ok {
		grade.Semester = semester
	}

	if uploadedBy, ok := doc["uploaded_by"].(string); ok {
		grade.UploadedBy = uploadedBy
	}

	// Timestamps
	if uploadedAt, ok := doc["uploaded_at"].(primitive.DateTime); ok {
		grade.UploadedAt = timestamppb.New(uploadedAt.Time())
	}

	if publishedAt, ok := doc["published_at"].(primitive.DateTime); ok {
		grade.PublishedAt = timestamppb.New(publishedAt.Time())
	}

	// Boolean fields
	if published, ok := doc["published"].(bool); ok {
		grade.Published = published
	}

	if overrideReason, ok := doc["override_reason"].(string); ok {
		grade.OverrideReason = overrideReason
	}

	return grade, nil
}

// calculateStudentGPA calculates GPA for a student
func (s *GradeService) calculateStudentGPA(ctx context.Context, studentID, semester string) (*pb.GPACalculation, error) {
	// Build filter for published grades
	filter := bson.M{
		"student_id": studentID,
		"published":  true,
		"grade": bson.M{
			"$in": []string{"A", "B", "C", "D", "F"},
		},
	}

	if semester != "" {
		filter["semester"] = semester
	}

	// Aggregate to calculate GPA
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": "$semester",
			"total_points": bson.M{
				"$sum": bson.M{
					"$multiply": []interface{}{
						bson.M{
							"$switch": bson.M{
								"branches": []bson.M{
									{"case": bson.M{"$eq": []interface{}{"$grade", "A"}}, "then": 4.0},
									{"case": bson.M{"$eq": []interface{}{"$grade", "B"}}, "then": 3.0},
									{"case": bson.M{"$eq": []interface{}{"$grade", "C"}}, "then": 2.0},
									{"case": bson.M{"$eq": []interface{}{"$grade", "D"}}, "then": 1.0},
									{"case": bson.M{"$eq": []interface{}{"$grade", "F"}}, "then": 0.0},
								},
								"default": 0.0,
							},
						},
						"$units",
					},
				},
			},
			"total_units":  bson.M{"$sum": "$units"},
			"course_count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := s.gradesCol.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregation failed: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		Semester    string  `bson:"_id"`
		TotalPoints float64 `bson:"total_points"`
		TotalUnits  float64 `bson:"total_units"`
		CourseCount int32   `bson:"course_count"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	// Calculate overall and per-semester GPA
	var semesterBreakdown []*pb.SemesterGPA
	var overallPoints, overallUnits float64
	var overallCourses int32

	for _, result := range results {
		semesterGPA := 0.0
		if result.TotalUnits > 0 {
			semesterGPA = result.TotalPoints / result.TotalUnits
		}

		semesterBreakdown = append(semesterBreakdown, &pb.SemesterGPA{
			Semester:     result.Semester,
			Gpa:          semesterGPA,
			Units:        int32(result.TotalUnits),
			CoursesCount: result.CourseCount,
		})

		overallPoints += result.TotalPoints
		overallUnits += result.TotalUnits
		overallCourses += result.CourseCount
	}

	overallGPA := 0.0
	if overallUnits > 0 {
		overallGPA = overallPoints / overallUnits
	}

	return &pb.GPACalculation{
		TermGpa:             overallGPA,
		Cgpa:                overallGPA, // For simplicity, same as term GPA in this calculation
		TotalUnitsAttempted: int32(overallUnits),
		TotalUnitsEarned:    int32(overallUnits),
		SemesterBreakdown:   semesterBreakdown,
	}, nil
}

// getStudentRosterEntry creates a StudentRosterEntry for a student
func (s *GradeService) getStudentRosterEntry(ctx context.Context, studentID, enrollmentID string) (*pb.StudentRosterEntry, error) {
	// Get student details
	var user struct {
		Name      string `bson:"name"`
		Email     string `bson:"email"`
		Major     string `bson:"major"`
		YearLevel int32  `bson:"year_level"`
	}

	err := s.usersCol.FindOne(ctx, bson.M{"_id": studentID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("failed to get student details: %w", err)
	}

	// Get grade if exists
	var grade string
	var gradeDoc struct {
		Grade string `bson:"grade"`
	}

	err = s.gradesCol.FindOne(ctx, bson.M{"enrollment_id": enrollmentID}).Decode(&gradeDoc)
	if err == nil {
		grade = gradeDoc.Grade
	}

	return &pb.StudentRosterEntry{
		StudentId:   studentID,
		StudentName: user.Name,
		Email:       user.Email,
		Major:       user.Major,
		YearLevel:   user.YearLevel,
		Grade:       grade,
	}, nil
}

// validateFacultyForCourse checks if a faculty is assigned to a course
func (s *GradeService) validateFacultyForCourse(ctx context.Context, courseID, facultyID string) error {
	// Check if faculty exists and has correct role
	var faculty struct {
		ID   string `bson:"_id"`
		Role string `bson:"role"`
	}

	err := s.usersCol.FindOne(ctx, bson.M{"_id": facultyID}).Decode(&faculty)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("faculty not found: %s", facultyID)
		}
		return fmt.Errorf("failed to retrieve faculty: %w", err)
	}

	if faculty.Role != "faculty" {
		return fmt.Errorf("user is not a faculty member: %s", facultyID)
	}

	// Check if faculty is assigned to this course
	var course struct {
		ID        string `bson:"_id"`
		FacultyID string `bson:"faculty_id"`
	}

	err = s.coursesCol.FindOne(ctx, bson.M{"_id": courseID}).Decode(&course)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("course not found: %s", courseID)
		}
		return fmt.Errorf("failed to retrieve course: %w", err)
	}

	if course.FacultyID != facultyID {
		return fmt.Errorf("faculty %s is not assigned to course %s", facultyID, courseID)
	}

	return nil
}

// uploadSingleGrade uploads a single grade entry
func (s *GradeService) uploadSingleGrade(ctx context.Context, courseID, facultyID string, entry *pb.GradeEntry) error {
	// Validate grade
	validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true, "I": true, "W": true}
	grade := strings.ToUpper(entry.Grade)
	if !validGrades[grade] {
		return fmt.Errorf("invalid grade: %s", entry.Grade)
	}

	// Find enrollment for this student in this course
	var enrollment struct {
		ID     string `bson:"_id"`
		Status string `bson:"status"`
	}

	err := s.enrollmentsCol.FindOne(ctx, bson.M{
		"student_id": entry.StudentId,
		"course_id":  courseID,
		"status":     "completed",
	}).Decode(&enrollment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("student %s not enrolled in course %s", entry.StudentId, courseID)
		}
		return fmt.Errorf("failed to find enrollment: %w", err)
	}

	// Check if grade already exists
	var existingGrade struct {
		ID string `bson:"_id"`
	}

	err = s.gradesCol.FindOne(ctx, bson.M{"enrollment_id": enrollment.ID}).Decode(&existingGrade)
	if err == nil {
		// Grade exists, update it
		update := bson.M{
			"$set": bson.M{
				"grade":            grade,
				"last_modified_by": facultyID,
				"last_modified_at": time.Now(),
				"override_reason":  "Grade updated via upload",
			},
		}

		_, err = s.gradesCol.UpdateOne(
			ctx,
			bson.M{"enrollment_id": enrollment.ID},
			update,
		)
		return err
	}

	// Get course and student info for denormalization
	var course struct {
		Code     string `bson:"code"`
		Title    string `bson:"title"`
		Units    int32  `bson:"units"`
		Semester string `bson:"semester"`
	}

	err = s.coursesCol.FindOne(ctx, bson.M{"_id": courseID}).Decode(&course)
	if err != nil {
		return fmt.Errorf("failed to get course info: %w", err)
	}

	var student struct {
		Name string `bson:"name"`
	}

	err = s.usersCol.FindOne(ctx, bson.M{"_id": entry.StudentId}).Decode(&student)
	if err != nil {
		return fmt.Errorf("failed to get student info: %w", err)
	}

	// Create new grade document
	gradeDoc := bson.M{
		"enrollment_id":   enrollment.ID,
		"student_id":      entry.StudentId,
		"student_name":    student.Name,
		"course_id":       courseID,
		"course_code":     course.Code,
		"course_title":    course.Title,
		"units":           course.Units,
		"grade":           grade,
		"semester":        course.Semester,
		"uploaded_by":     facultyID,
		"uploaded_at":     time.Now(),
		"published":       false,
		"override_reason": "",
	}

	_, err = s.gradesCol.InsertOne(ctx, gradeDoc)
	return err
}
