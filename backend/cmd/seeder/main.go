package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"stdiscm_p4/backend/internal/shared"
)

// Define the test constants based on the previous user accounts
const (
	// User IDs
	AdminID1   = "admin-001"
	FacultyID1 = "faculty-001"
	FacultyID2 = "faculty-002"
	StudentID1 = "student-001" // John Student, student@example.com
	StudentID2 = "student-002" // Alice Wonderland, student2@example.com
	StudentID3 = "student-003" // Bob Builder, student3@example.com

	// Common Credentials
	CommonPassword = "password"

	// Current Academic Period
	CurrentSemester  = "Fall 2024"
	PreviousSemester = "Spring 2024"

	// Course IDs
	CS101ID   = "CS101_Fall24"
	CS201ID   = "CS201_Fall24"
	MATH101ID = "MATH101_Fall24"
	HIS101ID  = "HIS101_Fall24"
)

// Course data structure for easy seeding
type CourseSeed struct {
	ID        string
	Code      string
	Title     string
	Units     int32
	Capacity  int32
	FacultyID string
	Schedule  string
	IsOpen    bool
	Semester  string
	PrereqID  string // Used to create a prerequisite entry later
}

// Enrollment data structure for easy seeding
type EnrollmentSeed struct {
	StudentID  string
	CourseID   string
	Status     string
	Semester   string
	Grade      string // Only for Status = "completed"
	FacultyID  string // Faculty who taught the course
	EnrolledAt time.Time
}

func main() {
	log.Println("Starting Comprehensive Database Seeder...")

	if err := shared.LoadEnv(".env"); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	cfg, err := shared.LoadServiceConfig("seeder")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	client, db, err := shared.ConnectMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer shared.DisconnectMongoDB(client)

	// Drop all collections to ensure a clean start for relational testing
	if err := db.Drop(context.Background()); err != nil {
		log.Fatalf("Failed to drop database: %v", err)
	}
	log.Println("Database cleared successfully.")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// --- 1. Seed Users ---
	seedUsers(ctx, db)

	// --- 2. Seed Courses and Prerequisites ---
	courseSeeds := []CourseSeed{
		{CS101ID, "CS-101", "Introduction to Programming", 3, 50, FacultyID1, "MWF 9:00-10:00", true, CurrentSemester, ""},
		{CS201ID, "CS-201", "Data Structures & Algorithms", 3, 30, FacultyID1, "TTH 14:00-15:30", true, CurrentSemester, "CS101_Spring24"}, // Requires old CS101
		{MATH101ID, "MATH-101", "Calculus I", 4, 60, FacultyID2, "MW 11:00-12:30", true, CurrentSemester, ""},
		{HIS101ID, "HIS-101", "World History", 3, 45, FacultyID2, "MWF 13:00-14:00", true, CurrentSemester, ""},
		// Completed course (must be created to exist for enrollment history/prereqs)
		{"CS101_Spring24", "CS-101", "Intro to Programming", 3, 50, FacultyID1, "MWF 9:00-10:00", false, PreviousSemester, ""},
		{"MATH101_Spring24", "MATH-101", "Calculus I", 4, 60, FacultyID2, "MW 11:00-12:30", false, PreviousSemester, ""},
	}
	seededCourses := seedCourses(ctx, db, courseSeeds)
	seedPrerequisites(ctx, db, CS201ID, "CS101_Spring24") // CS201 requires old CS101

	// --- 3. Seed Enrollments and Grades (History & Current) ---
	enrollmentSeeds := []EnrollmentSeed{
		// History: Student 1 Passed CS101 in Spring 2024 (Prereq for CS201 met)
		{StudentID1, "CS101_Spring24", shared.StatusCompleted, PreviousSemester, shared.GradeA, FacultyID1, time.Now().AddDate(0, -6, 0)},
		// History: Student 2 Passed MATH101 in Spring 2024
		{StudentID2, "MATH101_Spring24", shared.StatusCompleted, PreviousSemester, shared.GradeB, FacultyID2, time.Now().AddDate(0, -6, 0)},

		// Current: Student 1 is ENROLLED in MATH101 (Current Semester)
		{StudentID1, MATH101ID, shared.StatusEnrolled, CurrentSemester, "", FacultyID2, time.Now().AddDate(0, 0, -5)},
		// Current: Student 2 is ENROLLED in CS101 (Current Semester)
		{StudentID2, CS101ID, shared.StatusEnrolled, CurrentSemester, "", FacultyID1, time.Now().AddDate(0, 0, -4)},
		// Current: Student 3 is ENROLLED in CS101 and HIS101 (Current Semester)
		{StudentID3, CS101ID, shared.StatusEnrolled, CurrentSemester, "", FacultyID1, time.Now().AddDate(0, 0, -3)},
		{StudentID3, HIS101ID, shared.StatusEnrolled, CurrentSemester, "", FacultyID2, time.Now().AddDate(0, 0, -2)},
	}
	seedEnrollmentsAndGrades(ctx, db, enrollmentSeeds, seededCourses)

	// --- 4. Seed System Config ---
	seedSystemConfig(ctx, db)

	// --- 5. Final Enrollment Count Update (Important for Course Availability) ---
	updateCourseEnrollmentCounts(ctx, db)

	log.Println("All data seeding completed successfully.")
}

// ============================================================================
// SEEDING FUNCTIONS
// ============================================================================

func seedUsers(ctx context.Context, db *mongo.Database) {
	log.Println("--- Seeding Users ---")
	usersCol := db.Collection("users")

	users := []shared.User{
		{ID: AdminID1, Name: "Super Admin", Email: "admin@example.com", Role: shared.RoleAdmin, IsActive: true, CreatedAt: time.Now()},
		{ID: FacultyID1, Name: "Dr. Jane Professor", Email: "faculty@example.com", Role: shared.RoleFaculty, IsActive: true, CreatedAt: time.Now(), FacultyID: "FAC-001", Department: "Computer Science"},
		{ID: FacultyID2, Name: "Prof. Alan Turing", Email: "faculty2@example.com", Role: shared.RoleFaculty, IsActive: true, CreatedAt: time.Now(), FacultyID: "FAC-002", Department: "Mathematics"},
		{ID: StudentID1, Name: "John Student", Email: "student@example.com", Role: shared.RoleStudent, IsActive: true, CreatedAt: time.Now(), StudentID: "202400001", Major: "Computer Science", YearLevel: 1},
		{ID: StudentID2, Name: "Alice Wonderland", Email: "student2@example.com", Role: shared.RoleStudent, IsActive: true, CreatedAt: time.Now(), StudentID: "202400002", Major: "Information Systems", YearLevel: 2},
		{ID: StudentID3, Name: "Bob Builder", Email: "student3@example.com", Role: shared.RoleStudent, IsActive: true, CreatedAt: time.Now(), StudentID: "202400003", Major: "Computer Science", YearLevel: 3},
	}

	hashedBytes, _ := bcrypt.GenerateFromPassword([]byte(CommonPassword), 10)
	hashedPassword := string(hashedBytes)

	for _, u := range users {
		u.PasswordHash = hashedPassword
		filter := bson.M{"email": u.Email}
		update := bson.M{"$set": u}
		opts := options.Update().SetUpsert(true)

		_, err := usersCol.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Fatalf("Error seeding user %s: %v", u.Email, err)
		}
		log.Printf("Seeded %s: %s", u.Role, u.Email)
	}
}

func seedCourses(ctx context.Context, db *mongo.Database, seeds []CourseSeed) map[string]shared.Course {
	log.Println("--- Seeding Courses ---")
	coursesCol := db.Collection("courses")

	seededCourses := make(map[string]shared.Course)
	now := time.Now()

	for _, s := range seeds {
		course := shared.Course{
			ID:        s.ID,
			Code:      s.Code,
			Title:     s.Title,
			Units:     s.Units,
			Schedule:  s.Schedule,
			Room:      "SCI-101",
			Capacity:  s.Capacity,
			Enrolled:  0, // Will be updated by enrollment seeding
			FacultyID: s.FacultyID,
			IsOpen:    s.IsOpen,
			Semester:  s.Semester,
			CreatedAt: now,
			UpdatedAt: now,
		}

		_, err := coursesCol.InsertOne(ctx, course)
		if err != nil {
			log.Fatalf("Error seeding course %s: %v", s.Code, err)
		}
		log.Printf("Seeded Course: %s (%s)", s.Code, s.ID)
		seededCourses[s.ID] = course
	}
	return seededCourses
}

func seedPrerequisites(ctx context.Context, db *mongo.Database, courseID, prereqID string) {
	log.Println("--- Seeding Prerequisites ---")
	prereqsCol := db.Collection("prerequisites")

	prereq := shared.Prerequisite{CourseID: courseID, PrereqID: prereqID}

	_, err := prereqsCol.InsertOne(ctx, prereq)
	if err != nil {
		log.Fatalf("Error seeding prereq: %v", err)
	}
	log.Printf("Seeded Prereq: %s requires %s", courseID, prereqID)
}

func seedEnrollmentsAndGrades(ctx context.Context, db *mongo.Database, seeds []EnrollmentSeed, courses map[string]shared.Course) {
	log.Println("--- Seeding Enrollments & Grades ---")
	enrollmentsCol := db.Collection("enrollments")
	gradesCol := db.Collection("grades")

	for i, s := range seeds {
		course := courses[s.CourseID]
		enrollmentID := fmt.Sprintf("ENR-%d-%s", i+1, s.StudentID)
		now := time.Now()

		// 1. Create Enrollment
		enrollment := shared.Enrollment{
			ID:         enrollmentID,
			StudentID:  s.StudentID,
			CourseID:   s.CourseID,
			Status:     s.Status,
			EnrolledAt: s.EnrolledAt,
		}

		// Parse schedule info from course for the enrollment document
		days, start, end := shared.ParseSchedule(course.Schedule)
		enrollment.ScheduleInfo = shared.ScheduleInfo{
			Days: days, StartTime: start, EndTime: end,
		}

		_, err := enrollmentsCol.InsertOne(ctx, enrollment)
		if err != nil {
			log.Fatalf("Error seeding enrollment %s: %v", enrollmentID, err)
		}
		log.Printf("Seeded Enrollment: %s in %s (Status: %s)", s.StudentID, course.Code, s.Status)

		// 2. Create Grade (If Completed)
		if s.Status == shared.StatusCompleted && s.Grade != "" {
			// Find student name (simplified lookup for seeder)
			var student shared.User
			db.Collection("users").FindOne(ctx, bson.M{"_id": s.StudentID}).Decode(&student)

			grade := shared.Grade{
				EnrollmentID: enrollmentID,
				Grade:        s.Grade,
				UploadedBy:   s.FacultyID,
				UploadedAt:   now.Add(time.Hour),
				Published:    true, // Make published so it counts toward GPA/prereqs
				PublishedAt:  now.Add(2 * time.Hour),
				// Denormalized fields for GradeService's GetStudentGrades RPC
				// NOTE: Grade model in shared/models.go needs to be updated
				// to support these denormalized fields if not already done.
				// Assuming denormalized fields exist as per Grade PB definition.
				// The actual Grade struct in `shared/models.go` is minimal, so we'll use a BSON map.
			}

			// Manually construct document to include denormalized fields required by GradeService logic
			gradeDoc := bson.M{
				"enrollment_id": grade.EnrollmentID,
				"grade":         grade.Grade,
				"uploaded_by":   grade.UploadedBy,
				"uploaded_at":   primitive.NewDateTimeFromTime(grade.UploadedAt),
				"published":     grade.Published,
				"published_at":  primitive.NewDateTimeFromTime(grade.PublishedAt),
				// Denormalized fields for faster lookup and consistency
				"student_id":   student.ID,
				"student_name": student.Name,
				"course_id":    course.ID,
				"course_code":  course.Code,
				"course_title": course.Title,
				"units":        course.Units,
				"semester":     course.Semester,
			}

			_, err := gradesCol.InsertOne(ctx, gradeDoc)
			if err != nil {
				log.Fatalf("Error seeding grade %s for %s: %v", s.Grade, s.StudentID, err)
			}
			log.Printf("Seeded Grade: %s for %s (Published: True)", s.Grade, s.StudentID)
		}
	}
}

func seedSystemConfig(ctx context.Context, db *mongo.Database) {
	log.Println("--- Seeding System Config ---")
	configCol := db.Collection("system_config")
	now := time.Now()

	configs := []shared.SystemConfig{
		{Key: shared.ConfigCurrentSemester, Value: CurrentSemester, UpdatedAt: now, UpdatedBy: AdminID1, Description: "Current active academic semester"},
		{Key: shared.ConfigEnrollmentStart, Value: now.AddDate(0, -1, 0).Format(time.RFC3339), UpdatedAt: now, UpdatedBy: AdminID1, Description: "Start date of enrollment period"},
		{Key: shared.ConfigEnrollmentEnd, Value: now.AddDate(0, 1, 0).Format(time.RFC3339), UpdatedAt: now, UpdatedBy: AdminID1, Description: "End date of enrollment period"},
		{Key: "enrollment_enabled", Value: "true", UpdatedAt: now, UpdatedBy: AdminID1, Description: "Flag to enable/disable enrollment system-wide"},
	}

	for _, c := range configs {
		filter := bson.M{"key": c.Key}
		update := bson.M{"$set": c}
		opts := options.Update().SetUpsert(true)

		_, err := configCol.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Fatalf("Error seeding config %s: %v", c.Key, err)
		}
		log.Printf("Seeded Config: %s = %s", c.Key, c.Value)
	}
}

func updateCourseEnrollmentCounts(ctx context.Context, db *mongo.Database) {
	log.Println("--- Updating Course Enrollment Counts ---")

	// Aggregation pipeline to count active enrollments per course
	pipeline := []bson.M{
		{"$match": bson.M{"status": shared.StatusEnrolled}},
		{"$group": bson.M{
			"_id":   "$course_id",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := db.Collection("enrollments").Aggregate(ctx, pipeline)
	if err != nil {
		log.Fatalf("Error during enrollment aggregation: %v", err)
	}
	defer cursor.Close(ctx)

	counts := make(map[string]int32)
	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int32  `bson:"count"`
		}
		if err := cursor.Decode(&result); err == nil {
			counts[result.ID] = result.Count
		}
	}

	coursesCol := db.Collection("courses")
	for courseID, count := range counts {
		_, err := coursesCol.UpdateOne(ctx,
			bson.M{"_id": courseID},
			bson.M{"$set": bson.M{"enrolled": count}},
		)
		if err != nil {
			log.Printf("Warning: Failed to update enrolled count for %s: %v", courseID, err)
		} else {
			log.Printf("Updated Course %s: Enrolled=%d", courseID, count)
		}
	}

	// FIX: Convert []string (getKeys(counts)) to primitive.A explicitly for MongoDB query.
	courseIDsWithEnrollment := getKeys(counts)

	_, err = coursesCol.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$nin": courseIDsWithEnrollment}},
		bson.M{"$set": bson.M{"enrolled": 0}},
	)
	if err != nil {
		log.Fatalf("Error resetting enrollment counts: %v", err)
	}
}

func getKeys(m map[string]int32) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ============================================================================
// USER CREATION LOGIC (from previous turn)
// Note: This logic is now merged into the comprehensive 'seedUsers' function.
// ============================================================================

// The `seedUsers` function is defined above to align with the new main function.
