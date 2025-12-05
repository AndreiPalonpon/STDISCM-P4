package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"stdiscm_p4/backend/internal/gateway/handlers"
	"stdiscm_p4/backend/internal/gateway/util"
	pb_auth "stdiscm_p4/backend/internal/pb/auth"
)

// SetupRoutes configures the Chi router, middleware, and route handlers.
func SetupRoutes(clients *ServiceClients) *chi.Mux {
	r := chi.NewRouter()

	// 1. Global Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS Configuration (Allow React Frontend)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"}, // React default ports
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 2. Initialize Handlers
	authHandler := &handlers.AuthHandler{AuthClient: clients.AuthClient}
	courseHandler := &handlers.CourseHandler{CourseClient: clients.CourseClient}
	enrollmentHandler := &handlers.EnrollmentHandler{EnrollmentClient: clients.EnrollmentClient}
	gradeHandler := &handlers.GradeHandler{GradeClient: clients.GradeClient}
	adminHandler := &handlers.AdminHandler{AdminClient: clients.AdminClient}

	// 3. Define Routes (grouped by prefix)
	r.Route("/api", func(r chi.Router) {

		// --- Public Routes ---

		// Auth
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/logout", authHandler.Logout) // Logout handles its own token extraction, safe to be public-ish

		// Course Catalog (Publicly viewable)
		r.Get("/courses", courseHandler.ListCourses)
		r.Get("/courses/{id}", courseHandler.GetCourse)
		r.Get("/courses/{id}/availability", courseHandler.GetCourseAvailability)

		// --- Protected Routes (Require Valid Token) ---
		r.Group(func(r chi.Router) {
			// Inject Auth Middleware
			r.Use(AuthMiddleware(clients.AuthClient))

			// Auth (Authenticated Only)
			r.Get("/auth/validate", authHandler.ValidateToken)
			r.Post("/auth/change-password", authHandler.ChangePassword)

			// Course Prerequisites (Requires Student ID from token)
			r.Get("/courses/{id}/prerequisites", courseHandler.CheckPrerequisites)

			// Enrollment (Student Only)
			r.Route("/cart", func(r chi.Router) {
				r.Get("/", enrollmentHandler.GetCart)
				r.Post("/add", enrollmentHandler.AddToCart)
				r.Delete("/remove/{course_id}", enrollmentHandler.RemoveFromCart)
				r.Delete("/clear", enrollmentHandler.ClearCart)
			})
			r.Route("/enrollment", func(r chi.Router) {
				r.Post("/enroll-all", enrollmentHandler.EnrollAll)
				r.Post("/drop", enrollmentHandler.DropCourse)
				r.Get("/schedule", enrollmentHandler.GetStudentEnrollments)
			})

			// Grade Management
			r.Route("/grades", func(r chi.Router) {
				// Student
				r.Get("/", gradeHandler.GetStudentGrades)
				r.Get("/gpa", gradeHandler.CalculateGPA)

				// Faculty
				r.Get("/roster/{course_id}", gradeHandler.GetClassRoster)
				r.Get("/course/{course_id}", gradeHandler.GetCourseGrades)
				r.Post("/upload/{course_id}", gradeHandler.UploadGrades)
				r.Post("/publish/{course_id}", gradeHandler.PublishGrades)
			})

			// Admin Management
			r.Route("/admin", func(r chi.Router) {
				r.Get("/stats", adminHandler.GetSystemStats)
				r.Get("/config", adminHandler.GetSystemConfig)
				r.Put("/config/{key}", adminHandler.UpdateSystemConfig)

				// Courses
				r.Post("/courses", adminHandler.CreateCourse)
				r.Put("/courses/{id}", adminHandler.UpdateCourse)
				r.Delete("/courses/{id}", adminHandler.DeleteCourse)
				r.Post("/courses/{id}/assign-faculty", adminHandler.AssignFaculty)

				// Users
				r.Post("/users", adminHandler.CreateUser)
				r.Get("/users", adminHandler.ListUsers)
				r.Post("/users/{id}/reset-password", adminHandler.ResetPassword)
				r.Patch("/users/{id}/status", adminHandler.ToggleUserStatus)

				// Enrollment Config
				r.Post("/enrollment/period", adminHandler.SetEnrollmentPeriod)
				r.Post("/enrollment/toggle", adminHandler.ToggleEnrollment)

				// Overrides
				r.Post("/override/enroll", adminHandler.OverrideEnroll)
				r.Post("/override/drop", adminHandler.OverrideDrop)
			})
		})
	})

	return r
}

// AuthMiddleware creates a middleware that validates JWT tokens via the Auth Service.
func AuthMiddleware(authClient pb_auth.AuthServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract Token
			tokenStr, err := util.ExtractToken(r)
			if err != nil {
				util.WriteJSONError(w, http.StatusUnauthorized, "Authorization token required")
				return
			}

			// 2. Validate via gRPC
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			validateReq := &pb_auth.ValidateTokenRequest{Token: tokenStr}
			validateResp, err := authClient.ValidateToken(ctx, validateReq)

			if err != nil {
				// If service is down or error occurred
				util.HandleGRPCError(w, err)
				return
			}

			if !validateResp.Valid {
				util.WriteJSONError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			// 3. Inject User into Context
			// The handlers can now access user details via r.Context().Value("user")
			ctxWithUser := context.WithValue(r.Context(), "user", validateResp.User)
			next.ServeHTTP(w, r.WithContext(ctxWithUser))
		})
	}
}
