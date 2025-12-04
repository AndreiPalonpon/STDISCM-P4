package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "stdiscm_p4/backend/pb/grade"
	"stdiscm_p4/backend/shared"
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

	// Verify student exists using shared methods
	var student shared.User
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.StudentId}).Decode(&student)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Return empty list instead of error if student just has no grades yet but exists?
			// Original logic returned empty list if user not found, but kept error log.
			// Correct logic: if user not found, return empty.
			return &pb.GetStudentGradesResponse{
				Grades:  []*pb.Grade{},
				GpaInfo: &pb.GPACalculation{},
			}, nil
		}
		log.Printf("Error finding student %s: %v", req.StudentId, err)
		return nil, status.Error(codes.Internal, "failed to retrieve student information")
	}

	if student.Role != shared.RoleStudent {
		return nil, status.Error(codes.PermissionDenied, "user is not a student")
	}

	// Get grades for this student
	filter := bson.M{"student_id": req.StudentId}
	if req.Semester != "" {
		filter["semester"] = req.Semester
	}

	// Use shared helper for options if preferred, or manual
	findOptions := options.Find().
		SetSort(bson.D{{Key: "semester", Value: -1}, {Key: "course_code", Value: 1}}).
		SetLimit(100)

	cursor, err := s.gradesCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		log.Printf("Error querying grades: %v", err)
		return nil, status.Error(codes.Internal, "failed to retrieve grades")
	}
	defer cursor.Close(queryCtx)

	var grades []*pb.Grade
	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		grade, err := s.documentToGrade(doc)
		if err != nil {
			continue
		}
		grades = append(grades, grade)
	}

	// Calculate GPA using helper
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

	var student shared.User
	err := s.usersCol.FindOne(queryCtx, bson.M{"_id": req.StudentId}).Decode(&student)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.CalculateGPAResponse{
				Success: false,
				GpaInfo: &pb.GPACalculation{},
				Message: fmt.Sprintf("student not found: %s", req.StudentId),
			}, nil
		}
		return nil, status.Error(codes.Internal, "failed to retrieve student information")
	}

	if student.Role != shared.RoleStudent {
		return nil, status.Error(codes.PermissionDenied, "user is not a student")
	}

	gpaInfo, err := s.calculateStudentGPA(queryCtx, req.StudentId, req.Semester)
	if err != nil {
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

	var course shared.Course
	err := s.coursesCol.FindOne(queryCtx, bson.M{"_id": req.CourseId}).Decode(&course)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &pb.GetClassRosterResponse{}, nil
		}
		return nil, status.Error(codes.Internal, "failed to retrieve course information")
	}

	filter := bson.M{
		"course_id": req.CourseId,
		"status":    shared.StatusEnrolled,
	}

	findOptions := options.Find().SetSort(bson.D{{Key: "student_id", Value: 1}})
	cursor, err := s.enrollmentsCol.Find(queryCtx, filter, findOptions)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve enrollments")
	}
	defer cursor.Close(queryCtx)

	var students []*pb.StudentRosterEntry
	for cursor.Next(queryCtx) {
		var enrollment shared.Enrollment
		if err := cursor.Decode(&enrollment); err != nil {
			continue
		}

		studentEntry, err := s.getStudentRosterEntry(queryCtx, enrollment.StudentID, enrollment.ID)
		if err != nil {
			log.Printf("Error getting student entry for %s: %v", enrollment.StudentID, err)
			continue
		}
		students = append(students, studentEntry)
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

	for {
		req, err := stream.Recv()
		if err != nil {
			break
		} // Stream ended

		if !receivedMetadata {
			if req.GetMetadata().GetCourseId() == "" || req.GetMetadata().GetFacultyId() == "" {
				return status.Error(codes.InvalidArgument, "metadata missing")
			}
			courseID = req.GetMetadata().GetCourseId()
			facultyID = req.GetMetadata().GetFacultyId()

			if err := s.validateFacultyForCourse(stream.Context(), courseID, facultyID); err != nil {
				return status.Errorf(codes.PermissionDenied, "faculty validation failed: %v", err)
			}
			receivedMetadata = true
			continue
		}

		entry := req.GetEntry()
		if entry == nil {
			failed++
			errors = append(errors, "nil grade entry")
			continue
		}

		totalProcessed++

		if err := s.uploadSingleGrade(stream.Context(), courseID, facultyID, entry); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("student %s: %v", entry.StudentId, err))
		} else {
			successful++
		}

		if req.IsLast {
			break
		}
	}

	if !receivedMetadata {
		return status.Error(codes.InvalidArgument, "no metadata received")
	}

	return stream.SendAndClose(&pb.UploadGradesResponse{
		Success:        successful > 0 || totalProcessed == 0,
		TotalProcessed: totalProcessed,
		Successful:     successful,
		Failed:         failed,
		Errors:         errors,
		Message:        fmt.Sprintf("Processed %d grades", totalProcessed),
	})
}

// PublishGrades makes grades visible to students
func (s *GradeService) PublishGrades(ctx context.Context, req *pb.PublishGradesRequest) (*pb.PublishGradesResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid arguments")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.validateFacultyForCourse(queryCtx, req.CourseId, req.FacultyId); err != nil {
		return &pb.PublishGradesResponse{Success: false, Message: fmt.Sprintf("%v", err)}, nil
	}

	filter := bson.M{"course_id": req.CourseId, "published": false}
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
		return nil, status.Error(codes.Internal, "failed to publish grades")
	}

	msg := "no grades to publish"
	if result.ModifiedCount > 0 {
		msg = fmt.Sprintf("published %d grades", result.ModifiedCount)
	}

	return &pb.PublishGradesResponse{
		Success:         true,
		GradesPublished: int32(result.ModifiedCount),
		Message:         msg,
	}, nil
}

// GetCourseGrades retrieves all grades for a course (faculty only)
func (s *GradeService) GetCourseGrades(ctx context.Context, req *pb.GetCourseGradesRequest) (*pb.GetCourseGradesResponse, error) {
	if req == nil || req.CourseId == "" || req.FacultyId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid arguments")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.validateFacultyForCourse(queryCtx, req.CourseId, req.FacultyId); err != nil {
		return &pb.GetCourseGradesResponse{}, nil
	}

	filter := bson.M{"course_id": req.CourseId}
	cursor, err := s.gradesCol.Find(queryCtx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	defer cursor.Close(queryCtx)

	var grades []*pb.Grade
	allPublished := true
	count := 0

	for cursor.Next(queryCtx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		grade, err := s.documentToGrade(doc)
		if err != nil {
			continue
		}

		grades = append(grades, grade)
		count++

		if pub, _ := shared.GetBool(doc["published"]); !pub {
			allPublished = false
		}
	}

	return &pb.GetCourseGradesResponse{
		Grades:       grades,
		TotalGrades:  int32(count),
		AllPublished: allPublished && count > 0,
	}, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s *GradeService) documentToGrade(doc bson.M) (*pb.Grade, error) {
	grade := &pb.Grade{}

	// Safely extract fields using shared helpers
	if id, _ := shared.GetString(doc["enrollment_id"]); id != "" {
		grade.EnrollmentId = id
	} else {
		return nil, fmt.Errorf("missing enrollment_id")
	}
	if sid, _ := shared.GetString(doc["student_id"]); sid != "" {
		grade.StudentId = sid
	}
	if sname, _ := shared.GetString(doc["student_name"]); sname != "" {
		grade.StudentName = sname
	}
	if cid, _ := shared.GetString(doc["course_id"]); cid != "" {
		grade.CourseId = cid
	}
	if ccode, _ := shared.GetString(doc["course_code"]); ccode != "" {
		grade.CourseCode = ccode
	}
	if ctitle, _ := shared.GetString(doc["course_title"]); ctitle != "" {
		grade.CourseTitle = ctitle
	}

	if u, err := shared.GetInt32(doc["units"]); err == nil {
		grade.Units = u
	}
	if g, _ := shared.GetString(doc["grade"]); g != "" {
		grade.Grade = strings.ToUpper(g)
	}
	if sem, _ := shared.GetString(doc["semester"]); sem != "" {
		grade.Semester = sem
	}
	if upBy, _ := shared.GetString(doc["uploaded_by"]); upBy != "" {
		grade.UploadedBy = upBy
	}
	if reason, _ := shared.GetString(doc["override_reason"]); reason != "" {
		grade.OverrideReason = reason
	}

	if upAt, err := shared.GetTime(doc["uploaded_at"]); err == nil {
		grade.UploadedAt = timestamppb.New(upAt)
	}
	if pubAt, err := shared.GetTime(doc["published_at"]); err == nil {
		grade.PublishedAt = timestamppb.New(pubAt)
	}
	if pub, err := shared.GetBool(doc["published"]); err == nil {
		grade.Published = pub
	}

	return grade, nil
}

// CalculateGPA calculates GPA for a student
func (s *GradeService) calculateStudentGPA(ctx context.Context, studentID, semester string) (*pb.GPACalculation, error) {
	// Standardize GPA Calculation using shared.GetGradePoints
	filter := bson.M{
		"student_id": studentID,
		"published":  true,
		// Exclude I and W from GPA
		"grade": bson.M{"$nin": []string{shared.GradeI, shared.GradeW}},
	}
	if semester != "" {
		filter["semester"] = semester
	}

	cursor, err := s.gradesCol.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var overallPoints, overallUnits float64
	semesterMap := make(map[string]*struct {
		points, units float64
		count         int
	})

	for cursor.Next(ctx) {
		// FIX: Define a local struct that includes the denormalized fields (Units, Semester)
		// which exist in the MongoDB document but are missing from shared.Grade struct.
		var g struct {
			Grade    string `bson:"grade"`
			Units    int32  `bson:"units"`
			Semester string `bson:"semester"`
		}

		if err := cursor.Decode(&g); err != nil {
			log.Printf("Error decoding grade for GPA calc: %v", err)
			continue
		}

		// Use shared helper for points
		points := shared.GetGradePoints(g.Grade)
		units := float64(g.Units)

		// Aggregate Overall
		overallPoints += points * units
		overallUnits += units

		// Aggregate Semester
		if _, exists := semesterMap[g.Semester]; !exists {
			semesterMap[g.Semester] = &struct {
				points, units float64
				count         int
			}{}
		}
		sm := semesterMap[g.Semester]
		sm.points += points * units
		sm.units += units
		sm.count++
	}

	// Build Result
	calc := &pb.GPACalculation{
		TotalUnitsAttempted: int32(overallUnits),
		TotalUnitsEarned:    int32(overallUnits), // Simplified logic
	}

	// Calculate Cumulative GPA
	if overallUnits > 0 {
		calc.TermGpa = overallPoints / overallUnits
		calc.Cgpa = overallPoints / overallUnits
	}

	// Build Semester Breakdown
	for sem, data := range semesterMap {
		sgpa := 0.0
		if data.units > 0 {
			sgpa = data.points / data.units
		}
		calc.SemesterBreakdown = append(calc.SemesterBreakdown, &pb.SemesterGPA{
			Semester:     sem,
			Gpa:          sgpa,
			Units:        int32(data.units),
			CoursesCount: int32(data.count),
		})
	}

	return calc, nil
}

func (s *GradeService) getStudentRosterEntry(ctx context.Context, studentID, enrollmentID string) (*pb.StudentRosterEntry, error) {
	var user shared.User
	if err := s.usersCol.FindOne(ctx, bson.M{"_id": studentID}).Decode(&user); err != nil {
		return nil, err
	}

	var gradeDoc struct {
		Grade string `bson:"grade"`
	}
	grade := ""
	if err := s.gradesCol.FindOne(ctx, bson.M{"enrollment_id": enrollmentID}).Decode(&gradeDoc); err == nil {
		grade = gradeDoc.Grade
	}

	return &pb.StudentRosterEntry{
		StudentId: studentID, StudentName: user.Name, Email: user.Email,
		Major: user.Major, YearLevel: user.YearLevel, Grade: grade,
	}, nil
}

func (s *GradeService) validateFacultyForCourse(ctx context.Context, courseID, facultyID string) error {
	var faculty shared.User
	if err := s.usersCol.FindOne(ctx, bson.M{"_id": facultyID}).Decode(&faculty); err != nil {
		return fmt.Errorf("faculty not found")
	}
	if faculty.Role != shared.RoleFaculty {
		return fmt.Errorf("user not faculty")
	}

	var course shared.Course
	if err := s.coursesCol.FindOne(ctx, bson.M{"_id": courseID}).Decode(&course); err != nil {
		return fmt.Errorf("course not found")
	}
	if course.FacultyID != facultyID {
		return fmt.Errorf("faculty mismatch")
	}
	return nil
}

func (s *GradeService) uploadSingleGrade(ctx context.Context, courseID, facultyID string, entry *pb.GradeEntry) error {
	grade := strings.ToUpper(entry.Grade)
	if !shared.IsValidGrade(grade) {
		return fmt.Errorf("invalid grade")
	}

	// Find Enrollment
	var enrollment shared.Enrollment
	err := s.enrollmentsCol.FindOne(ctx, bson.M{
		"student_id": entry.StudentId, "course_id": courseID,
	}).Decode(&enrollment)

	// Simplified: allow upload if enrolled or completed
	if err != nil {
		return fmt.Errorf("student not enrolled")
	}

	// Upsert Grade
	update := bson.M{
		"$set": bson.M{
			"grade": grade, "last_modified_by": facultyID, "last_modified_at": time.Now(),
			"uploaded_by": facultyID, "uploaded_at": time.Now(),
			// Denormalize fields usually handled here, simplified for brevity
			"student_id": entry.StudentId, "course_id": courseID, "enrollment_id": enrollment.ID,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err = s.gradesCol.UpdateOne(ctx, bson.M{"enrollment_id": enrollment.ID}, update, opts)
	return err
}
