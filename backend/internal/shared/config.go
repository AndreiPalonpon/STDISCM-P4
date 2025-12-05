// ============================================================================
// backend/shared/config.go
// Shared configuration management and environment variable helpers
// ============================================================================

package shared

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// ============================================================================
// Configuration Structs
// ============================================================================

// ServiceConfig holds common configuration for all services
type ServiceConfig struct {
	ServiceName string
	ServicePort string
	Environment string // development, staging, production
	LogLevel    string // debug, info, warn, error

	// MongoDB Configuration
	MongoDB MongoConfig

	// gRPC Configuration
	GRPC GRPCConfig

	// Security Configuration
	Security SecurityConfig
}

// GRPCConfig holds gRPC-specific configuration
type GRPCConfig struct {
	MaxRecvMsgSize    int // Maximum receive message size in bytes
	MaxSendMsgSize    int // Maximum send message size in bytes
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	JWTSecret          string
	JWTExpirationHours int
	SessionTimeout     time.Duration
	BCryptCost         int // BCrypt hashing cost (10-12 recommended)
}

// GatewayConfig holds gateway-specific configuration
type GatewayConfig struct {
	ServiceConfig
	HTTPPort string

	// Service addresses
	AuthServiceAddr       string
	CourseServiceAddr     string
	EnrollmentServiceAddr string
	GradeServiceAddr      string
	AdminServiceAddr      string

	// CORS Configuration
	CORS CORSConfig
}

// CORSConfig holds CORS-related configuration
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int // in seconds
}

// ============================================================================
// Configuration Loading Functions
// ============================================================================

// LoadEnv loads environment variables from .env file
func LoadEnv(envFile string) error {
	if envFile == "" {
		envFile = ".env"
	}

	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Warning: %s file not found, using system environment variables", envFile)
		return err
	}

	log.Printf("Successfully loaded environment from %s", envFile)
	return nil
}

// LoadServiceConfig loads common service configuration from environment
func LoadServiceConfig(serviceName string) (*ServiceConfig, error) {
	config := &ServiceConfig{
		ServiceName: serviceName,
		ServicePort: GetEnv("SERVICE_PORT", "50051"),
		Environment: GetEnv("ENVIRONMENT", "development"),
		LogLevel:    GetEnv("LOG_LEVEL", "info"),
	}

	// Load MongoDB configuration
	mongoURI := GetEnv("MONGO_URI", "")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI environment variable is required")
	}

	config.MongoDB = MongoConfig{
		URI:            mongoURI,
		Database:       GetEnv("MONGO_DB_NAME", "ProblemSet4"),
		ConnectTimeout: GetDurationEnv("MONGO_CONNECT_TIMEOUT", 20*time.Second),
		MaxPoolSize:    uint64(GetIntEnv("MONGO_MAX_POOL_SIZE", 50)),
		MinPoolSize:    uint64(GetIntEnv("MONGO_MIN_POOL_SIZE", 10)),
		MaxIdleTime:    GetDurationEnv("MONGO_MAX_IDLE_TIME", 30*time.Second),
	}

	// Load gRPC configuration
	config.GRPC = GRPCConfig{
		MaxRecvMsgSize:    GetIntEnv("GRPC_MAX_RECV_MSG_SIZE", 10*1024*1024), // 10MB
		MaxSendMsgSize:    GetIntEnv("GRPC_MAX_SEND_MSG_SIZE", 10*1024*1024), // 10MB
		ConnectionTimeout: GetDurationEnv("GRPC_CONNECTION_TIMEOUT", 10*time.Second),
		RequestTimeout:    GetDurationEnv("GRPC_REQUEST_TIMEOUT", 30*time.Second),
	}

	// Load security configuration
	config.Security = SecurityConfig{
		JWTSecret:          GetEnv("JWT_SECRET", ""),
		JWTExpirationHours: GetIntEnv("JWT_EXPIRATION_HOURS", 24),
		SessionTimeout:     GetDurationEnv("SESSION_TIMEOUT", 30*time.Minute),
		BCryptCost:         GetIntEnv("BCRYPT_COST", 10),
	}

	// Validate required fields
	if config.Security.JWTSecret == "" && serviceName == "auth-service" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required for auth service")
	}

	return config, nil
}

// LoadGatewayConfig loads gateway-specific configuration
func LoadGatewayConfig() (*GatewayConfig, error) {
	baseConfig, err := LoadServiceConfig("gateway")
	if err != nil {
		return nil, err
	}

	config := &GatewayConfig{
		ServiceConfig: *baseConfig,
		HTTPPort:      GetEnv("HTTP_PORT", "8080"),

		// Service addresses
		AuthServiceAddr:       GetEnv("AUTH_SERVICE_ADDR", "localhost:50051"),
		CourseServiceAddr:     GetEnv("COURSE_SERVICE_ADDR", "localhost:50052"),
		EnrollmentServiceAddr: GetEnv("ENROLLMENT_SERVICE_ADDR", "localhost:50053"),
		GradeServiceAddr:      GetEnv("GRADE_SERVICE_ADDR", "localhost:50054"),
		AdminServiceAddr:      GetEnv("ADMIN_SERVICE_ADDR", "localhost:50055"),
	}

	// Load CORS configuration
	config.CORS = CORSConfig{
		AllowedOrigins:   GetStringSliceEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		AllowedMethods:   GetStringSliceEnv("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		AllowedHeaders:   GetStringSliceEnv("CORS_ALLOWED_HEADERS", []string{"Content-Type", "Authorization"}),
		AllowCredentials: GetBoolEnv("CORS_ALLOW_CREDENTIALS", true),
		MaxAge:           GetIntEnv("CORS_MAX_AGE", 3600),
	}

	return config, nil
}

// ============================================================================
// Environment Variable Helper Functions
// ============================================================================

// GetEnv retrieves an environment variable or returns a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetIntEnv retrieves an integer environment variable or returns a default value
func GetIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid integer value for %s: %s, using default: %d", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}

// GetBoolEnv retrieves a boolean environment variable or returns a default value
func GetBoolEnv(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid boolean value for %s: %s, using default: %t", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}

// GetDurationEnv retrieves a duration environment variable or returns a default value
// Supports format like "30s", "5m", "1h"
func GetDurationEnv(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid duration value for %s: %s, using default: %v", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}

// GetStringSliceEnv retrieves a comma-separated string list or returns a default value
func GetStringSliceEnv(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	// Split by comma
	var result []string
	current := ""
	for _, char := range valueStr {
		if char == ',' {
			if trimmed := trimSpace(current); trimmed != "" {
				result = append(result, trimmed)
			}
			current = ""
		} else {
			current += string(char)
		}
	}

	// Add last item
	if trimmed := trimSpace(current); trimmed != "" {
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return defaultValue
	}

	return result
}

// trimSpace removes leading and trailing spaces
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Trim leading spaces
	for start < len(s) && s[start] == ' ' {
		start++
	}

	// Trim trailing spaces
	for end > start && s[end-1] == ' ' {
		end--
	}

	return s[start:end]
}

// ============================================================================
// Configuration Validation
// ============================================================================

// ValidateServiceConfig validates service configuration
func ValidateServiceConfig(config *ServiceConfig) error {
	if config.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if config.ServicePort == "" {
		return fmt.Errorf("service port is required")
	}

	if config.MongoDB.URI == "" {
		return fmt.Errorf("MongoDB URI is required")
	}

	if config.MongoDB.Database == "" {
		return fmt.Errorf("MongoDB database name is required")
	}

	return nil
}

// ValidateGatewayConfig validates gateway configuration
func ValidateGatewayConfig(config *GatewayConfig) error {
	if err := ValidateServiceConfig(&config.ServiceConfig); err != nil {
		return err
	}

	if config.HTTPPort == "" {
		return fmt.Errorf("HTTP port is required")
	}

	if config.AuthServiceAddr == "" {
		return fmt.Errorf("auth service address is required")
	}

	if config.CourseServiceAddr == "" {
		return fmt.Errorf("course service address is required")
	}

	if config.EnrollmentServiceAddr == "" {
		return fmt.Errorf("enrollment service address is required")
	}

	if config.GradeServiceAddr == "" {
		return fmt.Errorf("grade service address is required")
	}

	if config.AdminServiceAddr == "" {
		return fmt.Errorf("admin service address is required")
	}

	return nil
}

// ============================================================================
// Configuration Display (for debugging)
// ============================================================================

// PrintConfig prints configuration (sanitized) for debugging
func PrintConfig(config *ServiceConfig) {
	log.Println("=== Service Configuration ===")
	log.Printf("Service Name: %s", config.ServiceName)
	log.Printf("Service Port: %s", config.ServicePort)
	log.Printf("Environment: %s", config.Environment)
	log.Printf("Log Level: %s", config.LogLevel)
	log.Println("=== MongoDB Configuration ===")
	log.Printf("Database: %s", config.MongoDB.Database)
	log.Printf("Max Pool Size: %d", config.MongoDB.MaxPoolSize)
	log.Printf("Min Pool Size: %d", config.MongoDB.MinPoolSize)
	log.Println("=== gRPC Configuration ===")
	log.Printf("Max Recv Msg Size: %d bytes", config.GRPC.MaxRecvMsgSize)
	log.Printf("Max Send Msg Size: %d bytes", config.GRPC.MaxSendMsgSize)
	log.Printf("Connection Timeout: %v", config.GRPC.ConnectionTimeout)
	log.Println("=== Security Configuration ===")
	log.Printf("JWT Expiration: %d hours", config.Security.JWTExpirationHours)
	log.Printf("Session Timeout: %v", config.Security.SessionTimeout)
	log.Printf("BCrypt Cost: %d", config.Security.BCryptCost)
	log.Println("=============================")
}

// PrintGatewayConfig prints gateway configuration (sanitized)
func PrintGatewayConfig(config *GatewayConfig) {
	PrintConfig(&config.ServiceConfig)
	log.Println("=== Gateway-Specific Configuration ===")
	log.Printf("HTTP Port: %s", config.HTTPPort)
	log.Printf("Auth Service: %s", config.AuthServiceAddr)
	log.Printf("Course Service: %s", config.CourseServiceAddr)
	log.Printf("Enrollment Service: %s", config.EnrollmentServiceAddr)
	log.Printf("Grade Service: %s", config.GradeServiceAddr)
	log.Printf("Admin Service: %s", config.AdminServiceAddr)
	log.Println("=== CORS Configuration ===")
	log.Printf("Allowed Origins: %v", config.CORS.AllowedOrigins)
	log.Printf("Allowed Methods: %v", config.CORS.AllowedMethods)
	log.Printf("Allow Credentials: %t", config.CORS.AllowCredentials)
	log.Println("======================================")
}

// ============================================================================
// Default Port Mapping
// ============================================================================

const (
	DefaultGatewayHTTPPort       = "8080"
	DefaultAuthServicePort       = "50051"
	DefaultCourseServicePort     = "50052"
	DefaultEnrollmentServicePort = "50053"
	DefaultGradeServicePort      = "50054"
	DefaultAdminServicePort      = "50055"
)

// GetServicePort returns the default port for a service
func GetServicePort(serviceName string) string {
	ports := map[string]string{
		"gateway":            DefaultGatewayHTTPPort,
		"auth-service":       DefaultAuthServicePort,
		"course-service":     DefaultCourseServicePort,
		"enrollment-service": DefaultEnrollmentServicePort,
		"grade-service":      DefaultGradeServicePort,
		"admin-service":      DefaultAdminServicePort,
	}

	if port, exists := ports[serviceName]; exists {
		return port
	}

	return "50051" // Default gRPC port
}

// ============================================================================
// Configuration File Support (Optional)
// ============================================================================

// ConfigFile represents a configuration file structure
type ConfigFile struct {
	Service  ServiceFileConfig  `json:"service"`
	MongoDB  MongoFileConfig    `json:"mongodb"`
	GRPC     GRPCFileConfig     `json:"grpc"`
	Security SecurityFileConfig `json:"security"`
}

// ServiceFileConfig represents service config in file
type ServiceFileConfig struct {
	Name        string `json:"name"`
	Port        string `json:"port"`
	Environment string `json:"environment"`
	LogLevel    string `json:"log_level"`
}

// MongoFileConfig represents MongoDB config in file
type MongoFileConfig struct {
	URI         string `json:"uri"`
	Database    string `json:"database"`
	MaxPoolSize int    `json:"max_pool_size"`
	MinPoolSize int    `json:"min_pool_size"`
}

// GRPCFileConfig represents gRPC config in file
type GRPCFileConfig struct {
	MaxRecvMsgSize int    `json:"max_recv_msg_size"`
	MaxSendMsgSize int    `json:"max_send_msg_size"`
	Timeout        string `json:"timeout"`
}

// SecurityFileConfig represents security config in file
type SecurityFileConfig struct {
	JWTExpirationHours int    `json:"jwt_expiration_hours"`
	SessionTimeout     string `json:"session_timeout"`
	BCryptCost         int    `json:"bcrypt_cost"`
}

// ============================================================================
// Environment-Specific Configuration
// ============================================================================

// IsDevelopment checks if running in development environment
func IsDevelopment(config *ServiceConfig) bool {
	return config.Environment == "development"
}

// IsProduction checks if running in production environment
func IsProduction(config *ServiceConfig) bool {
	return config.Environment == "production"
}

// IsStaging checks if running in staging environment
func IsStaging(config *ServiceConfig) bool {
	return config.Environment == "staging"
}

// GetLogLevel returns the configured log level
func GetLogLevel(config *ServiceConfig) string {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if validLevels[config.LogLevel] {
		return config.LogLevel
	}

	return "info" // Default
}

// ============================================================================
// Required Environment Variables Check
// ============================================================================

// CheckRequiredEnvVars checks if all required environment variables are set
func CheckRequiredEnvVars(serviceName string) error {
	required := []string{
		"MONGO_URI",
		"MONGO_DB_NAME",
	}

	// Service-specific required variables
	switch serviceName {
	case "auth-service":
		required = append(required, "JWT_SECRET")
	case "gateway":
		required = append(required,
			"AUTH_SERVICE_ADDR",
			"COURSE_SERVICE_ADDR",
			"ENROLLMENT_SERVICE_ADDR",
			"GRADE_SERVICE_ADDR",
			"ADMIN_SERVICE_ADDR",
		)
	}

	var missing []string
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}
