// ============================================================================
// backend/shared/models.go
// Shared data models and structs for MongoDB documents
// ============================================================================

package shared

import (
	"time"
)

// ============================================================================
// User Models
// ============================================================================

// User represents a user account (student, faculty, or admin)
type User struct {
	ID           string    `bson:"_id" json:"id"`
	Email        string    `bson:"email" json:"email"`
	PasswordHash string    `bson:"password_hash" json:"-"` // Never expose in JSON
	Role         string    `bson:"role" json:"role"`       // student, faculty, admin
	Name         string    `bson:"name" json:"name"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at,omitempty" json:"updated_at,omitempty"`

	// Student-specific fields
	StudentID string `bson:"student_id,omitempty" json:"student_id,omitempty"`
	Major     string `bson:"major,omitempty" json:"major,omitempty"`
	YearLevel int32  `bson:"year_level,omitempty" json:"year_level,omitempty"`

	// Faculty-specific fields
	FacultyID  string `bson:"faculty_id,omitempty" json:"faculty_id,omitempty"`
	Department string `bson:"department,omitempty" json:"department,omitempty"`

	// Account status
	IsActive bool `bson:"is_active" json:"is_active"`
}

// Session represents an active user session (for JWT tracking)
type Session struct {
	ID        string    `bson:"_id" json:"id"`
	UserID    string    `bson:"user_id" json:"user_id"`
	Token     string    `bson:"token" json:"token"`
	ExpiresAt time.Time `bson:"expires_at" json:"expires_at"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	IPAddress string    `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
}

// ============================================================================
// Course Models
// ============================================================================

// Course represents a course offering
type Course struct {
	ID          string    `bson:"_id" json:"id"`
	Code        string    `bson:"code" json:"code"`
	Title       string    `bson:"title" json:"title"`
	Description string    `bson:"description,omitempty" json:"description,omitempty"`
	Units       int32     `bson:"units" json:"units"`
	Schedule    string    `bson:"schedule" json:"schedule"` // e.g., "MWF 9:00-10:00"
	Room        string    `bson:"room" json:"room"`
	Capacity    int32     `bson:"capacity" json:"capacity"`
	Enrolled    int32     `bson:"enrolled" json:"enrolled"`
	FacultyID   string    `bson:"faculty_id" json:"faculty_id"`
	IsOpen      bool      `bson:"is_open" json:"is_open"`
	Semester    string    `bson:"semester" json:"semester"` // e.g., "Spring 2024"
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// Prerequisite represents a prerequisite relationship between courses
type Prerequisite struct {
	CourseID string `bson:"course_id" json:"course_id"` // Course that requires prerequisite
	PrereqID string `bson:"prereq_id" json:"prereq_id"` // Prerequisite course
}

// ============================================================================
// Enrollment Models
// ============================================================================

// ScheduleInfo represents parsed schedule information
type ScheduleInfo struct {
	Days      []string `bson:"days" json:"days"`             // ["M", "W", "F"]
	StartTime string   `bson:"start_time" json:"start_time"` // "9:00"
	EndTime   string   `bson:"end_time" json:"end_time"`     // "10:00"
}

// Enrollment represents a student's enrollment in a course
type Enrollment struct {
	ID           string       `bson:"_id" json:"id"`
	StudentID    string       `bson:"student_id" json:"student_id"`
	CourseID     string       `bson:"course_id" json:"course_id"`
	Status       string       `bson:"status" json:"status"` // enrolled, dropped, completed
	EnrolledAt   time.Time    `bson:"enrolled_at" json:"enrolled_at"`
	DroppedAt    time.Time    `bson:"dropped_at,omitempty" json:"dropped_at,omitempty"`
	ScheduleInfo ScheduleInfo `bson:"schedule_info,omitempty" json:"schedule_info,omitempty"`
}

// Cart represents a student's shopping cart
type Cart struct {
	StudentID         string          `bson:"student_id" json:"student_id"`
	CourseIDs         []string        `bson:"course_ids" json:"course_ids"`
	UpdatedAt         time.Time       `bson:"updated_at" json:"updated_at"`
	ValidationResults *CartValidation `bson:"validation_results,omitempty" json:"validation_results,omitempty"`
}

// CartValidation stores validation results for a cart
type CartValidation struct {
	TotalUnits           int32    `bson:"total_units" json:"total_units"`
	HasConflicts         bool     `bson:"has_conflicts" json:"has_conflicts"`
	MissingPrerequisites []string `bson:"missing_prerequisites" json:"missing_prerequisites"`
}

// ============================================================================
// Grade Models
// ============================================================================

// Grade represents a student's grade for a course
type Grade struct {
	EnrollmentID   string    `bson:"enrollment_id" json:"enrollment_id"`
	Grade          string    `bson:"grade" json:"grade"` // A, B, C, D, F, I, W
	UploadedBy     string    `bson:"uploaded_by" json:"uploaded_by"`
	UploadedAt     time.Time `bson:"uploaded_at" json:"uploaded_at"`
	Published      bool      `bson:"published" json:"published"`
	PublishedAt    time.Time `bson:"published_at,omitempty" json:"published_at,omitempty"`
	OverrideReason string    `bson:"override_reason,omitempty" json:"override_reason,omitempty"`
	LastModifiedBy string    `bson:"last_modified_by,omitempty" json:"last_modified_by,omitempty"`
	LastModifiedAt time.Time `bson:"last_modified_at,omitempty" json:"last_modified_at,omitempty"`
}

// GradeEntry represents a single grade entry (for bulk upload)
type GradeEntry struct {
	StudentID string `json:"student_id"`
	Grade     string `json:"grade"`
}

// GPAInfo represents GPA calculation results
type GPAInfo struct {
	TermGPA             float64       `json:"term_gpa"`
	CGPA                float64       `json:"cgpa"`
	TotalUnitsAttempted int32         `json:"total_units_attempted"`
	TotalUnitsEarned    int32         `json:"total_units_earned"`
	SemesterBreakdown   []SemesterGPA `json:"semester_breakdown"`
}

// SemesterGPA represents GPA for a specific semester
type SemesterGPA struct {
	Semester     string  `json:"semester"`
	GPA          float64 `json:"gpa"`
	Units        int32   `json:"units"`
	CoursesCount int32   `json:"courses_count"`
}

// ============================================================================
// System Configuration Models
// ============================================================================

// SystemConfig represents a system configuration parameter
type SystemConfig struct {
	Key         string    `bson:"key" json:"key"`
	Value       string    `bson:"value" json:"value"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
	UpdatedBy   string    `bson:"updated_by,omitempty" json:"updated_by,omitempty"`
	Description string    `bson:"description,omitempty" json:"description,omitempty"`
}

// EnrollmentPeriod represents the enrollment period configuration
type EnrollmentPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsOpen    bool      `json:"is_open"`
}

// SystemStats represents system statistics for admin dashboard
type SystemStats struct {
	TotalStudents    int32  `json:"total_students"`
	TotalFaculty     int32  `json:"total_faculty"`
	TotalCourses     int32  `json:"total_courses"`
	OpenCourses      int32  `json:"open_courses"`
	TotalEnrollments int32  `json:"total_enrollments"`
	EnrollmentOpen   bool   `json:"enrollment_open"`
	CurrentSemester  string `json:"current_semester"`
}

// ============================================================================
// Audit Log Models
// ============================================================================

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        string                 `bson:"_id" json:"id"`
	Timestamp time.Time              `bson:"timestamp" json:"timestamp"`
	UserID    string                 `bson:"user_id" json:"user_id"`
	Action    string                 `bson:"action" json:"action"` // login, logout, enroll, drop, etc.
	Resource  string                 `bson:"resource" json:"resource"`
	Details   map[string]interface{} `bson:"details,omitempty" json:"details,omitempty"`
	IPAddress string                 `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
}

// ============================================================================
// Response Models (for API responses)
// ============================================================================

// CourseWithDetails extends Course with additional denormalized data
type CourseWithDetails struct {
	Course
	FacultyName    string   `json:"faculty_name,omitempty"`
	Prerequisites  []string `json:"prerequisites,omitempty"`
	SeatsAvailable int32    `json:"seats_available"`
}

// EnrollmentWithDetails extends Enrollment with denormalized course info
type EnrollmentWithDetails struct {
	Enrollment
	CourseCode  string `json:"course_code"`
	CourseTitle string `json:"course_title"`
	Units       int32  `json:"units"`
	Room        string `json:"room,omitempty"`
}

// GradeWithDetails extends Grade with denormalized course/student info
type GradeWithDetails struct {
	Grade
	StudentName string `json:"student_name"`
	CourseCode  string `json:"course_code"`
	CourseTitle string `json:"course_title"`
	Units       int32  `json:"units"`
	Semester    string `json:"semester"`
}

// StudentRosterEntry represents a student in a class roster
type StudentRosterEntry struct {
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Email       string `json:"email"`
	Major       string `json:"major,omitempty"`
	YearLevel   int32  `json:"year_level,omitempty"`
	Grade       string `json:"grade,omitempty"` // Current grade if uploaded
}

// ============================================================================
// Helper Methods
// ============================================================================

// GetGradePoints returns the grade point value for a letter grade
func GetGradePoints(grade string) float64 {
	gradePoints := map[string]float64{
		"A": 4.0,
		"B": 3.0,
		"C": 2.0,
		"D": 1.0,
		"F": 0.0,
		"I": 0.0, // Incomplete, not counted
		"W": 0.0, // Withdrawn, not counted
	}

	if points, exists := gradePoints[grade]; exists {
		return points
	}
	return 0.0
}

// IsPassingGrade checks if a grade is passing
func IsPassingGrade(grade string) bool {
	passingGrades := map[string]bool{
		"A": true,
		"B": true,
		"C": true,
		"D": true,
	}
	return passingGrades[grade]
}

// IsGradeCountedInGPA checks if grade should be counted in GPA calculation
func IsGradeCountedInGPA(grade string) bool {
	// I (Incomplete) and W (Withdrawn) are not counted
	return grade != "I" && grade != "W"
}

// GetSeatsAvailable calculates available seats for a course
func (c *Course) GetSeatsAvailable() int32 {
	available := c.Capacity - c.Enrolled
	if available < 0 {
		return 0
	}
	return available
}

// IsAvailable checks if a course is available for enrollment
func (c *Course) IsAvailable() bool {
	return c.IsOpen && c.GetSeatsAvailable() > 0
}

// IsCartFull checks if cart has reached maximum courses
func (c *Cart) IsCartFull() bool {
	return len(c.CourseIDs) >= 6
}

// CanAddCourse checks if a course can be added to cart
func (c *Cart) CanAddCourse(courseID string) bool {
	// Check if already in cart
	for _, id := range c.CourseIDs {
		if id == courseID {
			return false
		}
	}
	return !c.IsCartFull()
}

// IsExpired checks if a session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// ============================================================================
// Validation Constants
// ============================================================================

const (
	// Cart limits
	MaxCoursesInCart    = 6
	MaxUnitsPerSemester = 18

	// Enrollment statuses
	StatusEnrolled  = "enrolled"
	StatusDropped   = "dropped"
	StatusCompleted = "completed"

	// User roles
	RoleStudent = "student"
	RoleFaculty = "faculty"
	RoleAdmin   = "admin"

	// Grades
	GradeA = "A"
	GradeB = "B"
	GradeC = "C"
	GradeD = "D"
	GradeF = "F"
	GradeI = "I" // Incomplete
	GradeW = "W" // Withdrawn

	// Audit actions
	ActionLogin        = "login"
	ActionLogout       = "logout"
	ActionEnroll       = "enroll"
	ActionDrop         = "drop"
	ActionGradeUpload  = "grade_upload"
	ActionCourseCreate = "course_create"
	ActionCourseUpdate = "course_update"
	ActionUserCreate   = "user_create"
	ActionUserUpdate   = "user_update"
	ActionConfigChange = "config_change"

	// System config keys
	ConfigEnrollmentStart = "enrollment_start"
	ConfigEnrollmentEnd   = "enrollment_end"
	ConfigMaxUnits        = "max_units_per_semester"
	ConfigMaxCourses      = "max_courses_in_cart"
	ConfigCurrentSemester = "current_semester"
	ConfigGradeDeadline   = "grade_upload_deadline"
)

// ============================================================================
// Filter/Query Models
// ============================================================================

// CourseFilter represents filters for course queries
type CourseFilter struct {
	Department  string `json:"department,omitempty"`
	SearchQuery string `json:"search_query,omitempty"`
	OpenOnly    bool   `json:"open_only"`
	Semester    string `json:"semester,omitempty"`
	FacultyID   string `json:"faculty_id,omitempty"`
}

// EnrollmentFilter represents filters for enrollment queries
type EnrollmentFilter struct {
	StudentID string `json:"student_id,omitempty"`
	CourseID  string `json:"course_id,omitempty"`
	Semester  string `json:"semester,omitempty"`
	Status    string `json:"status,omitempty"`
}

// UserFilter represents filters for user queries
type UserFilter struct {
	Role       string `json:"role,omitempty"`
	ActiveOnly bool   `json:"active_only"`
	Department string `json:"department,omitempty"`
	Major      string `json:"major,omitempty"`
}

// ============================================================================
// Error Models
// ============================================================================

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ConflictInfo represents a schedule conflict
type ConflictInfo struct {
	Course1ID    string `json:"course1_id"`
	Course1Code  string `json:"course1_code"`
	Course2ID    string `json:"course2_id"`
	Course2Code  string `json:"course2_code"`
	ConflictType string `json:"conflict_type"` // "schedule" or "duplicate"
	Details      string `json:"details"`
}

// PrerequisiteCheckResult represents prerequisite check result
type PrerequisiteCheckResult struct {
	CourseID   string `json:"course_id"`
	CourseCode string `json:"course_code"`
	Met        bool   `json:"met"`
	Grade      string `json:"grade,omitempty"`
}
