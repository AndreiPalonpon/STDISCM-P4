package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"

	"stdiscm_p4/backend/pb/course"
	"stdiscm_p4/backend/pb/enrollment"
)

type EnrollmentService struct {
	enrollment.UnimplementedEnrollmentServiceServer
	db           *mongo.Client
	cartsColl    *mongo.Collection
	enrollColl   *mongo.Collection
	coursesColl  *mongo.Collection // Needed to update seat counts
	courseClient course.CourseServiceClient
}

func NewEnrollmentService(db *mongo.Client, cClient course.CourseServiceClient) *EnrollmentService {
	return &EnrollmentService{
		db:           db,
		cartsColl:    db.Database("college_db").Collection("carts"),
		enrollColl:   db.Database("college_db").Collection("enrollments"),
		coursesColl:  db.Database("college_db").Collection("courses"),
		courseClient: cClient,
	}
}

// ----------------------------------------------------------------------------
// Shopping Cart Methods
// ----------------------------------------------------------------------------

func (s *EnrollmentService) AddToCart(ctx context.Context, req *enrollment.AddToCartRequest) (*enrollment.AddToCartResponse, error) {
	// 1. Validate inputs
	if req.StudentId == "" || req.CourseId == "" {
		return &enrollment.AddToCartResponse{Success: false, Message: "Missing student_id or course_id"}, nil
	}

	// 2. Fetch Course Details (via gRPC to CourseService) to get code, title, units
	// We need these to store denormalized data in the cart
	courseRes, err := s.courseClient.GetCourse(ctx, &course.GetCourseRequest{CourseId: req.CourseId})
	if err != nil || !courseRes.Success {
		return &enrollment.AddToCartResponse{Success: false, Message: "Course not found"}, nil
	}
	c := courseRes.Course

	// 3. Add to Cart (Upsert)
	// We use $addToSet to prevent duplicates automatically
	item := bson.M{
		"course_id":    c.Id,
		"course_code":  c.Code,
		"course_title": c.Title,
		"units":        c.Units,
		"schedule":     c.Schedule, // Simplified for now
	}

	filter := bson.M{"student_id": req.StudentId}
	update := bson.M{
		"$addToSet": bson.M{"items": item},
		"$set":      bson.M{"updated_at": time.Now()},
	}
	opts := options.Update().SetUpsert(true)

	_, err = s.cartsColl.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return &enrollment.AddToCartResponse{Success: false, Message: "Failed to update cart"}, err
	}

	// 4. Return updated cart state (simplified: just success)
	// Ideally, we would fetch the cart and return it, but for speed we return success
	return &enrollment.AddToCartResponse{
		Success: true,
		Message: "Added to cart",
	}, nil
}

func (s *EnrollmentService) RemoveFromCart(ctx context.Context, req *enrollment.RemoveFromCartRequest) (*enrollment.RemoveFromCartResponse, error) {
	filter := bson.M{"student_id": req.StudentId}
	update := bson.M{
		"$pull": bson.M{"items": bson.M{"course_id": req.CourseId}},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	res, err := s.cartsColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return &enrollment.RemoveFromCartResponse{Success: false, Message: "Database error"}, err
	}

	if res.ModifiedCount == 0 {
		return &enrollment.RemoveFromCartResponse{Success: false, Message: "Item not found in cart"}, nil
	}

	return &enrollment.RemoveFromCartResponse{Success: true, Message: "Removed from cart"}, nil
}

func (s *EnrollmentService) GetCart(ctx context.Context, req *enrollment.GetCartRequest) (*enrollment.GetCartResponse, error) {
	var cartDoc struct {
		StudentID string `bson:"student_id"`
		Items     []struct {
			CourseID    string `bson:"course_id"`
			CourseCode  string `bson:"course_code"`
			CourseTitle string `bson:"course_title"`
			Units       int32  `bson:"units"`
		} `bson:"items"`
	}

	err := s.cartsColl.FindOne(ctx, bson.M{"student_id": req.StudentId}).Decode(&cartDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Return empty cart
			return &enrollment.GetCartResponse{
				Success: true,
				Cart:    &enrollment.Cart{StudentId: req.StudentId, Items: []*enrollment.CartItem{}},
			}, nil
		}
		return nil, err
	}

	// Convert BSON to Proto
	var pbItems []*enrollment.CartItem
	var totalUnits int32
	for _, it := range cartDoc.Items {
		pbItems = append(pbItems, &enrollment.CartItem{
			CourseId:    it.CourseID,
			CourseCode:  it.CourseCode,
			CourseTitle: it.CourseTitle,
			Units:       it.Units,
		})
		totalUnits += it.Units
	}

	return &enrollment.GetCartResponse{
		Success: true,
		Cart: &enrollment.Cart{
			StudentId:  req.StudentId,
			Items:      pbItems,
			TotalUnits: totalUnits,
		},
	}, nil
}

// ----------------------------------------------------------------------------
// Enrollment Logic
// ----------------------------------------------------------------------------

func (s *EnrollmentService) EnrollAll(ctx context.Context, req *enrollment.EnrollAllRequest) (*enrollment.EnrollAllResponse, error) {
	// 1. Get current cart
	cartRes, err := s.GetCart(ctx, &enrollment.GetCartRequest{StudentId: req.StudentId})
	if err != nil || !cartRes.Success || len(cartRes.Cart.Items) == 0 {
		return &enrollment.EnrollAllResponse{Success: false, Message: "Cart is empty"}, nil
	}

	items := cartRes.Cart.Items
	var failedCourses []string

	// 2. Validate ALL courses before enrolling any (Simplification: All or Nothing)
	// We check Prerequisites and Availability
	for _, item := range items {
		// A. Check Prerequisites
		prereqRes, err := s.courseClient.CheckPrerequisites(ctx, &course.CheckPrerequisitesRequest{
			StudentId: req.StudentId,
			CourseId:  item.CourseId,
		})
		if err != nil || !prereqRes.AllMet {
			failedCourses = append(failedCourses, item.CourseCode+" (Prereq missing)")
			continue
		}

		// B. Check Availability (Capacity)
		availRes, err := s.courseClient.GetCourseAvailability(ctx, &course.GetCourseAvailabilityRequest{CourseId: item.CourseId})
		if err != nil || !availRes.Available {
			failedCourses = append(failedCourses, item.CourseCode+" (Full)")
			continue
		}
	}

	if len(failedCourses) > 0 {
		return &enrollment.EnrollAllResponse{
			Success:       false,
			Message:       "Enrollment failed due to conflicts",
			FailedCourses: failedCourses,
		}, nil
	}

	// 3. Process Enrollment (Transaction-like)
	var enrollments []*enrollment.Enrollment
	for _, item := range items {
		// Insert into enrollments collection [cite: 647]
		enrollDoc := bson.M{
			"student_id":   req.StudentId,
			"course_id":    item.CourseId,
			"course_code":  item.CourseCode,
			"course_title": item.CourseTitle,
			"units":        item.Units,
			"status":       "enrolled",
			"enrolled_at":  time.Now(),
		}

		_, err := s.enrollColl.InsertOne(ctx, enrollDoc)
		if err != nil {
			log.Printf("Error inserting enrollment: %v", err)
			continue
		}

		// 4. Update Course Seat Count (Direct DB update for simplification)
		// Spec: "No real-time seat count", but we must track capacity [cite: 54]
		_, _ = s.coursesColl.UpdateOne(ctx,
			bson.M{"_id": item.CourseId},
			bson.M{"$inc": bson.M{"enrolled": 1}},
		)

		enrollments = append(enrollments, &enrollment.Enrollment{
			CourseCode: item.CourseCode,
			Status:     "enrolled",
		})
	}

	// 5. Clear Cart
	_, _ = s.cartsColl.DeleteOne(ctx, bson.M{"student_id": req.StudentId})

	return &enrollment.EnrollAllResponse{
		Success:     true,
		Message:     "Successfully enrolled in all courses",
		Enrollments: enrollments,
	}, nil
}

func (s *EnrollmentService) DropCourse(ctx context.Context, req *enrollment.DropCourseRequest) (*enrollment.DropCourseResponse, error) {
	// 1. Mark status as "dropped" in enrollments
	filter := bson.M{
		"student_id": req.StudentId,
		"course_id":  req.CourseId,
		"status":     "enrolled",
	}
	update := bson.M{
		"$set": bson.M{
			"status":     "dropped",
			"dropped_at": time.Now(),
		},
	}

	res, err := s.enrollColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return &enrollment.DropCourseResponse{Success: false, Message: "DB Error"}, err
	}
	if res.ModifiedCount == 0 {
		return &enrollment.DropCourseResponse{Success: false, Message: "Not enrolled in this course"}, nil
	}

	// 2. Decrement Course Count
	_, _ = s.coursesColl.UpdateOne(ctx,
		bson.M{"_id": req.CourseId},
		bson.M{"$inc": bson.M{"enrolled": -1}},
	)

	return &enrollment.DropCourseResponse{Success: true, Message: "Course dropped"}, nil
}

func (s *EnrollmentService) GetStudentEnrollments(ctx context.Context, req *enrollment.GetStudentEnrollmentsRequest) (*enrollment.GetStudentEnrollmentsResponse, error) {
	filter := bson.M{"student_id": req.StudentId}
	if req.Status != "" {
		filter["status"] = req.Status
	}

	cursor, err := s.enrollColl.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*enrollment.Enrollment
	var totalUnits int32

	for cursor.Next(ctx) {
		var doc struct {
			ID          primitive.ObjectID `bson:"_id"`
			CourseID    string             `bson:"course_id"`
			CourseCode  string             `bson:"course_code"`
			CourseTitle string             `bson:"course_title"`
			Units       int32              `bson:"units"`
			Status      string             `bson:"status"`
			EnrolledAt  time.Time          `bson:"enrolled_at"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		results = append(results, &enrollment.Enrollment{
			Id:          doc.ID.Hex(),
			CourseId:    doc.CourseID,
			CourseCode:  doc.CourseCode,
			CourseTitle: doc.CourseTitle,
			Units:       doc.Units,
			Status:      doc.Status,
			EnrolledAt:  timestamppb.New(doc.EnrolledAt),
		})

		if doc.Status == "enrolled" {
			totalUnits += doc.Units
		}
	}

	return &enrollment.GetStudentEnrollmentsResponse{
		Enrollments: results,
		TotalUnits:  totalUnits,
	}, nil
}

// Stubs for methods not critical for this demo
func (s *EnrollmentService) ClearCart(ctx context.Context, req *enrollment.ClearCartRequest) (*enrollment.ClearCartResponse, error) {
	_, err := s.cartsColl.DeleteOne(ctx, bson.M{"student_id": req.StudentId})
	return &enrollment.ClearCartResponse{Success: err == nil}, err
}

func (s *EnrollmentService) CheckConflicts(ctx context.Context, req *enrollment.CheckConflictsRequest) (*enrollment.CheckConflictsResponse, error) {
	return &enrollment.CheckConflictsResponse{HasConflicts: false}, nil
}
