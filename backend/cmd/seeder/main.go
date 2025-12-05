package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"

	// Ensure this matches your module name in go.mod
	"stdiscm_p4/backend/internal/shared"
)

func main() {
	log.Println("Starting Database Seeder...")

	// 1. Load Configuration
	if err := shared.LoadEnv(".env"); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	cfg, err := shared.LoadServiceConfig("seeder")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 2. Connect to MongoDB
	client, db, err := shared.ConnectMongoDB(&cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		if err := shared.DisconnectMongoDB(client); err != nil {
			log.Printf("Error disconnecting: %v", err)
		}
	}()

	usersCol := db.Collection("users")

	// 3. Define Dummy Users
	// We use the same password "password" for everyone for simplicity, as requested.
	commonPassword := "password"

	dummyUsers := []struct {
		User shared.User
	}{
		// --- ADMINS ---
		{
			User: shared.User{
				ID:        "admin-001",
				Name:      "Super Admin",
				Email:     "admin@example.com",
				Role:      shared.RoleAdmin,
				IsActive:  true,
				CreatedAt: time.Now(),
			},
		},
		{
			User: shared.User{
				ID:        "admin-002",
				Name:      "System Administrator",
				Email:     "admin2@example.com",
				Role:      shared.RoleAdmin,
				IsActive:  true,
				CreatedAt: time.Now(),
			},
		},

		// --- FACULTY ---
		// The specific account from README
		{
			User: shared.User{
				ID:         "faculty-001",
				Name:       "Dr. Jane Professor",
				Email:      "faculty@example.com",
				Role:       shared.RoleFaculty,
				IsActive:   true,
				CreatedAt:  time.Now(),
				FacultyID:  "FAC-001",
				Department: "Computer Science",
			},
		},
		// Additional faculty
		{
			User: shared.User{
				ID:         "faculty-002",
				Name:       "Prof. Alan Turing",
				Email:      "faculty2@example.com",
				Role:       shared.RoleFaculty,
				IsActive:   true,
				CreatedAt:  time.Now(),
				FacultyID:  "FAC-002",
				Department: "Mathematics",
			},
		},
		{
			User: shared.User{
				ID:         "faculty-003",
				Name:       "Dr. Grace Hopper",
				Email:      "faculty3@example.com",
				Role:       shared.RoleFaculty,
				IsActive:   true,
				CreatedAt:  time.Now(),
				FacultyID:  "FAC-003",
				Department: "Software Engineering",
			},
		},

		// --- STUDENTS ---
		// The specific account from README
		{
			User: shared.User{
				ID:        "student-001",
				Name:      "John Student",
				Email:     "student@example.com",
				Role:      shared.RoleStudent,
				IsActive:  true,
				CreatedAt: time.Now(),
				StudentID: "202400001",
				Major:     "Computer Science",
				YearLevel: 1,
			},
		},
		// Additional students
		{
			User: shared.User{
				ID:        "student-002",
				Name:      "Alice Wonderland",
				Email:     "student2@example.com",
				Role:      shared.RoleStudent,
				IsActive:  true,
				CreatedAt: time.Now(),
				StudentID: "202400002",
				Major:     "Information Systems",
				YearLevel: 2,
			},
		},
		{
			User: shared.User{
				ID:        "student-003",
				Name:      "Bob Builder",
				Email:     "student3@example.com",
				Role:      shared.RoleStudent,
				IsActive:  true,
				CreatedAt: time.Now(),
				StudentID: "202400003",
				Major:     "Computer Science",
				YearLevel: 3,
			},
		},
		{
			User: shared.User{
				ID:        "student-004",
				Name:      "Charlie Brown",
				Email:     "student4@example.com",
				Role:      shared.RoleStudent,
				IsActive:  true,
				CreatedAt: time.Now(),
				StudentID: "202400004",
				Major:     "Data Science",
				YearLevel: 4,
			},
		},
	}

	// 4. Insert/Update Users
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pre-calculate hash once since they all use the same password
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(commonPassword), 10)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	hashedPassword := string(hashedBytes)

	for _, entry := range dummyUsers {
		u := entry.User
		u.PasswordHash = hashedPassword

		// Upsert based on Email to avoid duplicates if run multiple times
		filter := bson.M{"email": u.Email}
		update := bson.M{"$set": u}
		opts := options.Update().SetUpsert(true)

		_, err := usersCol.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			log.Printf("Error seeding user %s: %v", u.Email, err)
		} else {
			log.Printf("Seeded %s: %s (Password: %s)", u.Role, u.Email, commonPassword)
		}
	}

	log.Println("Seeding completed successfully.")
}
