// ============================================================================
// backend/shared/database.go
// Shared MongoDB connection and helper utilities
// ============================================================================

package shared

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// MongoConfig holds MongoDB connection configuration
type MongoConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
	MaxPoolSize    uint64
	MinPoolSize    uint64
	MaxIdleTime    time.Duration
}

// DefaultMongoConfig returns default MongoDB configuration
func DefaultMongoConfig(uri, database string) *MongoConfig {
	return &MongoConfig{
		URI:            uri,
		Database:       database,
		ConnectTimeout: 20 * time.Second,
		MaxPoolSize:    50,
		MinPoolSize:    10,
		MaxIdleTime:    30 * time.Second,
	}
}

// ConnectMongoDB establishes connection to MongoDB Atlas/Local with proper configuration
func ConnectMongoDB(config *MongoConfig) (*mongo.Client, *mongo.Database, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("mongo config cannot be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectTimeout)
	defer cancel()

	// Configure client options for MongoDB Atlas
	clientOptions := options.Client().
		ApplyURI(config.URI).
		SetMaxPoolSize(config.MaxPoolSize).
		SetMinPoolSize(config.MinPoolSize).
		SetMaxConnIdleTime(config.MaxIdleTime).
		SetServerSelectionTimeout(10 * time.Second).
		SetConnectTimeout(config.ConnectTimeout).
		SetSocketTimeout(30 * time.Second).
		SetHeartbeatInterval(10 * time.Second)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping MongoDB to verify connection
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	log.Printf("Successfully connected to MongoDB (Database: %s)", config.Database)

	db := client.Database(config.Database)
	return client, db, nil
}

// DisconnectMongoDB gracefully closes MongoDB connection
func DisconnectMongoDB(client *mongo.Client) error {
	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
	}

	log.Println("Successfully disconnected from MongoDB")
	return nil
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// GetInt32 safely extracts int32 from BSON value (handles int32, int64, int)
func GetInt32(value interface{}) (int32, error) {
	switch v := value.(type) {
	case int32:
		return v, nil
	case int64:
		return int32(v), nil
	case int:
		return int32(v), nil
	case float64:
		return int32(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int32", value)
	}
}

// GetInt64 safely extracts int64 from BSON value
func GetInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

// GetString safely extracts string from BSON value
func GetString(value interface{}) (string, error) {
	if str, ok := value.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("cannot convert %T to string", value)
}

// GetBool safely extracts bool from BSON value
func GetBool(value interface{}) (bool, error) {
	if b, ok := value.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("cannot convert %T to bool", value)
}

// GetTime safely extracts time.Time from BSON DateTime
func GetTime(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case primitive.DateTime:
		return v.Time(), nil
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
	}
}

// GetStringArray safely extracts string array from BSON Array
func GetStringArray(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case primitive.A:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []string", value)
	}
}

// ============================================================================
// Schedule Parsing Helpers (for enrollment conflict detection)
// ============================================================================

// ParseSchedule extracts days, start time, and end time from schedule string
// Format: "MWF 9:00-10:00" or "TTH 14:00-15:30"
func ParseSchedule(schedule string) (days []string, startTime string, endTime string) {
	if schedule == "" {
		return []string{}, "", ""
	}

	// Split by space to separate days and times
	parts := splitBySpace(schedule)
	if len(parts) < 2 {
		return []string{}, "", ""
	}

	// Parse days (first part)
	daysStr := parts[0]
	days = parseDays(daysStr)

	// Parse times (second part, format: "HH:MM-HH:MM")
	timeRange := parts[1]
	times := splitByDash(timeRange)
	if len(times) == 2 {
		startTime = times[0]
		endTime = times[1]
	}

	return days, startTime, endTime
}

// parseDays converts day string to array (e.g., "MWF" -> ["M", "W", "F"])
func parseDays(daysStr string) []string {
	days := []string{}
	i := 0
	for i < len(daysStr) {
		// Check for two-letter day codes (TH, TTH)
		if i+1 < len(daysStr) && daysStr[i:i+2] == "TH" {
			days = append(days, "TH")
			i += 2
		} else {
			days = append(days, string(daysStr[i]))
			i++
		}
	}
	return days
}

// splitBySpace splits string by space
func splitBySpace(s string) []string {
	var result []string
	current := ""
	for _, char := range s {
		if char == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// splitByDash splits string by dash
func splitByDash(s string) []string {
	var result []string
	current := ""
	for _, char := range s {
		if char == '-' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// TimesOverlap checks if two time ranges overlap
// Format: "HH:MM" (e.g., "9:00", "10:30")
func TimesOverlap(start1, end1, start2, end2 string) bool {
	// Convert times to minutes since midnight for easy comparison
	s1 := timeToMinutes(start1)
	e1 := timeToMinutes(end1)
	s2 := timeToMinutes(start2)
	e2 := timeToMinutes(end2)

	// Check for overlap: (s1 < e2) AND (s2 < e1)
	return s1 < e2 && s2 < e1
}

// timeToMinutes converts "HH:MM" to minutes since midnight
func timeToMinutes(timeStr string) int {
	if timeStr == "" {
		return 0
	}

	parts := splitByColon(timeStr)
	if len(parts) != 2 {
		return 0
	}

	hours := 0
	minutes := 0

	// Parse hours
	for _, char := range parts[0] {
		if char >= '0' && char <= '9' {
			hours = hours*10 + int(char-'0')
		}
	}

	// Parse minutes
	for _, char := range parts[1] {
		if char >= '0' && char <= '9' {
			minutes = minutes*10 + int(char-'0')
		}
	}

	return hours*60 + minutes
}

// splitByColon splits string by colon
func splitByColon(s string) []string {
	var result []string
	current := ""
	for _, char := range s {
		if char == ':' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// DaysOverlap checks if two day arrays have any common days
func DaysOverlap(days1, days2 []string) bool {
	daySet := make(map[string]bool)
	for _, day := range days1 {
		daySet[day] = true
	}
	for _, day := range days2 {
		if daySet[day] {
			return true
		}
	}
	return false
}

// ============================================================================
// ID Generation Helpers
// ============================================================================

// GenerateID generates a unique ID with prefix and timestamp
func GenerateID(prefix string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d", prefix, timestamp)
}

// GenerateEnrollmentID generates enrollment ID
func GenerateEnrollmentID() string {
	return GenerateID("ENR")
}

// GenerateAuditLogID generates audit log ID
func GenerateAuditLogID() string {
	return GenerateID("AUDIT")
}

// ============================================================================
// Document Field Extraction Helpers
// ============================================================================

// ExtractScheduleInfo extracts schedule_info subdocument from course
func ExtractScheduleInfo(doc bson.M) bson.M {
	scheduleStr, err := GetString(doc["schedule"])
	if err != nil {
		return bson.M{}
	}

	days, startTime, endTime := ParseSchedule(scheduleStr)

	return bson.M{
		"days":       days,
		"start_time": startTime,
		"end_time":   endTime,
	}
}

// GetCourseField safely gets a field from course document
func GetCourseField(doc bson.M, field string) interface{} {
	if val, exists := doc[field]; exists {
		return val
	}
	return nil
}

// ============================================================================
// Audit Logging Helper
// ============================================================================

// LogAuditEvent logs an audit event to the audit_logs collection
func LogAuditEvent(ctx context.Context, auditCol *mongo.Collection, userID, action, resource string, details map[string]interface{}) error {
	if auditCol == nil {
		return fmt.Errorf("audit collection is nil")
	}

	auditDoc := bson.M{
		"_id":       GenerateAuditLogID(),
		"timestamp": primitive.NewDateTimeFromTime(time.Now()),
		"user_id":   userID,
		"action":    action,
		"resource":  resource,
	}

	if details != nil {
		auditDoc["details"] = details
	}

	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := auditCol.InsertOne(insertCtx, auditDoc)
	if err != nil {
		log.Printf("Warning: Failed to log audit event: %v", err)
		return err
	}

	return nil
}

// ============================================================================
// Query Helpers
// ============================================================================

// BuildFindOptions creates common find options with defaults
func BuildFindOptions(limit int64, sortField string, sortOrder int) *options.FindOptions {
	opts := options.Find()

	if limit > 0 {
		opts.SetLimit(limit)
	}

	if sortField != "" {
		opts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})
	}

	return opts
}

// CountDocumentsWithTimeout counts documents with timeout
func CountDocumentsWithTimeout(ctx context.Context, col *mongo.Collection, filter bson.M, timeout time.Duration) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	count, err := col.CountDocuments(queryCtx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	return count, nil
}

// FindOneWithTimeout finds a single document with timeout
func FindOneWithTimeout(ctx context.Context, col *mongo.Collection, filter bson.M, result interface{}, timeout time.Duration) error {
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := col.FindOne(queryCtx, filter).Decode(result)
	if err != nil {
		return err
	}

	return nil
}

// ============================================================================
// Transaction Helpers
// ============================================================================

// WithTransaction executes a function within a MongoDB transaction
func WithTransaction(ctx context.Context, client *mongo.Client, fn func(sessCtx mongo.SessionContext) error) error {
	session, err := client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	})

	return err
}

// ============================================================================
// Validation Helpers
// ============================================================================

// ValidateRequiredFields checks if required fields exist in document
func ValidateRequiredFields(doc bson.M, requiredFields []string) error {
	for _, field := range requiredFields {
		if _, exists := doc[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}

// IsValidGrade checks if grade is valid according to schema
func IsValidGrade(grade string) bool {
	validGrades := map[string]bool{
		"A": true, "B": true, "C": true, "D": true, "F": true, "I": true, "W": true,
	}
	return validGrades[grade]
}

// IsValidEnrollmentStatus checks if enrollment status is valid
func IsValidEnrollmentStatus(status string) bool {
	validStatuses := map[string]bool{
		"enrolled": true, "dropped": true, "completed": true,
	}
	return validStatuses[status]
}

// IsValidRole checks if user role is valid
func IsValidRole(role string) bool {
	validRoles := map[string]bool{
		"student": true, "faculty": true, "admin": true,
	}
	return validRoles[role]
}
