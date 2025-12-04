package enrollment

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options" // Added for UpdateOptions
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb_course "stdiscm_p4/backend/internal/pb/course"
	pb "stdiscm_p4/backend/internal/pb/enrollment"
	"stdiscm_p4/backend/internal/shared"
)

// EnrollmentService implements the gRPC EnrollmentService
type EnrollmentService struct {
	pb.UnimplementedEnrollmentServiceServer
	client         *mongo.Client
	db             *mongo.Database
	cartsCol       *mongo.Collection
	enrollmentsCol *mongo.Collection
	coursesCol     *mongo.Collection
	courseClient   pb_course.CourseServiceClient
}

// NewEnrollmentService creates a new EnrollmentService instance
func NewEnrollmentService(client *mongo.Client, db *mongo.Database, courseClient pb_course.CourseServiceClient) *EnrollmentService {
	return &EnrollmentService{
		client:         client,
		db:             db,
		cartsCol:       db.Collection("carts"),
		enrollmentsCol: db.Collection("enrollments"),
		coursesCol:     db.Collection("courses"),
		courseClient:   courseClient,
	}
}

// AddToCart adds a course to the student's shopping cart
func (s *EnrollmentService) AddToCart(ctx context.Context, req *pb.AddToCartRequest) (*pb.AddToCartResponse, error) {
	if req == nil || req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id and course_id are required")
	}

	// 1. Check if course exists and is open (via Course Service)
	courseResp, err := s.courseClient.GetCourse(ctx, &pb_course.GetCourseRequest{CourseId: req.CourseId})
	if err != nil || !courseResp.Success {
		return nil, status.Errorf(codes.NotFound, "course not found or unavailable")
	}
	if !courseResp.Course.IsOpen {
		return nil, status.Errorf(codes.FailedPrecondition, "course is closed for enrollment")
	}

	// 2. Get or Create Cart
	var cart shared.Cart
	err = s.cartsCol.FindOne(ctx, bson.M{"student_id": req.StudentId}).Decode(&cart)
	if err == mongo.ErrNoDocuments {
		// Initialize new cart
		cart = shared.Cart{
			StudentID: req.StudentId,
			CourseIDs: []string{},
		}
	} else if err != nil {
		return nil, status.Error(codes.Internal, "failed to retrieve cart")
	}

	// 3. Validation: Check max courses
	if cart.IsCartFull() {
		return nil, status.Errorf(codes.FailedPrecondition, "cart is full (max %d courses)", shared.MaxCoursesInCart)
	}

	// 4. Validation: Check duplicates
	if !cart.CanAddCourse(req.CourseId) {
		return nil, status.Errorf(codes.AlreadyExists, "course already in cart")
	}

	// 5. Update Cart
	update := bson.M{
		"$addToSet": bson.M{"course_ids": req.CourseId},
		"$set":      bson.M{"updated_at": time.Now()},
	}

	// FIX: Use options.Update() instead of shared.BuildFindOptions
	opts := options.Update().SetUpsert(true)

	_, err = s.cartsCol.UpdateOne(ctx, bson.M{"student_id": req.StudentId}, update, opts)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update cart")
	}

	// 6. Return updated cart details
	// FIX: Wrap the GetCart response into AddToCartResponse
	getCartResp, err := s.GetCart(ctx, &pb.GetCartRequest{StudentId: req.StudentId})
	if err != nil {
		return nil, err
	}

	return &pb.AddToCartResponse{
		Success: true,
		Message: "course added to cart",
		Cart:    getCartResp.Cart,
	}, nil
}

// RemoveFromCart removes a course from the cart
func (s *EnrollmentService) RemoveFromCart(ctx context.Context, req *pb.RemoveFromCartRequest) (*pb.RemoveFromCartResponse, error) {
	if req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid arguments")
	}

	_, err := s.cartsCol.UpdateOne(ctx,
		bson.M{"student_id": req.StudentId},
		bson.M{
			"$pull": bson.M{"course_ids": req.CourseId},
			"$set":  bson.M{"updated_at": time.Now()},
		},
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to remove from cart")
	}

	// FIX: Wrap the GetCart response into RemoveFromCartResponse
	getCartResp, err := s.GetCart(ctx, &pb.GetCartRequest{StudentId: req.StudentId})
	if err != nil {
		return nil, err
	}

	return &pb.RemoveFromCartResponse{
		Success: true,
		Message: "course removed from cart",
		Cart:    getCartResp.Cart,
	}, nil
}

// GetCart retrieves the current cart with full course details and validation
func (s *EnrollmentService) GetCart(ctx context.Context, req *pb.GetCartRequest) (*pb.GetCartResponse, error) {
	if req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id required")
	}

	// Fetch Cart
	var cartModel shared.Cart
	err := s.cartsCol.FindOne(ctx, bson.M{"student_id": req.StudentId}).Decode(&cartModel)
	if err == mongo.ErrNoDocuments {
		return &pb.GetCartResponse{
			Success: true,
			Cart:    &pb.Cart{StudentId: req.StudentId, Items: []*pb.CartItem{}},
			Message: "cart is empty",
		}, nil
	} else if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}

	// Hydrate Cart Items using Course Service
	var cartItems []*pb.CartItem
	var totalUnits int32
	var courseIDs []string

	for _, cid := range cartModel.CourseIDs {
		// Call Course Service
		cResp, err := s.courseClient.GetCourse(ctx, &pb_course.GetCourseRequest{CourseId: cid})
		if err != nil || !cResp.Success {
			log.Printf("Warning: Course %s in cart not found", cid)
			continue
		}

		course := cResp.Course
		courseIDs = append(courseIDs, cid)
		totalUnits += course.Units

		// Parse schedule for frontend display
		days, start, end := shared.ParseSchedule(course.Schedule)

		cartItems = append(cartItems, &pb.CartItem{
			CourseId:    course.Id,
			CourseCode:  course.Code,
			CourseTitle: course.Title,
			Units:       course.Units,
			ScheduleInfo: &pb.ScheduleInfo{
				Days:      days,
				StartTime: start,
				EndTime:   end,
			},
		})
	}

	// Check Conflicts locally
	conflicts := s.checkScheduleConflictsInternal(cartItems)
	hasConflicts := len(conflicts) > 0

	// FIX: Removed unused prereqResp variable block

	// Check missing prereqs for ALL items in cart
	var missingPrereqs []string
	for _, item := range cartItems {
		pResp, err := s.courseClient.CheckPrerequisites(ctx, &pb_course.CheckPrerequisitesRequest{
			StudentId: req.StudentId,
			CourseId:  item.CourseId,
		})
		if err == nil && !pResp.AllMet {
			missingPrereqs = append(missingPrereqs, item.CourseId)
		}
	}

	return &pb.GetCartResponse{
		Success: true,
		Cart: &pb.Cart{
			StudentId:            req.StudentId,
			Items:                cartItems,
			TotalUnits:           totalUnits,
			HasConflicts:         hasConflicts,
			MissingPrerequisites: missingPrereqs,
			UpdatedAt:            timestamppb.New(cartModel.UpdatedAt),
		},
		Message: "cart retrieved",
	}, nil
}

// ClearCart empties the student's cart
func (s *EnrollmentService) ClearCart(ctx context.Context, req *pb.ClearCartRequest) (*pb.ClearCartResponse, error) {
	_, err := s.cartsCol.DeleteOne(ctx, bson.M{"student_id": req.StudentId})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to clear cart")
	}
	return &pb.ClearCartResponse{Success: true, Message: "cart cleared"}, nil
}

// EnrollAll processes all items in the cart (Transactional)
func (s *EnrollmentService) EnrollAll(ctx context.Context, req *pb.EnrollAllRequest) (*pb.EnrollAllResponse, error) {
	if req.StudentId == "" {
		return nil, status.Error(codes.InvalidArgument, "student_id required")
	}

	// 1. Get Cart
	getCartResp, err := s.GetCart(ctx, &pb.GetCartRequest{StudentId: req.StudentId})
	if err != nil {
		return nil, err
	}
	cart := getCartResp.Cart
	if len(cart.Items) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "cart is empty")
	}

	// 2. Pre-Transaction Validation
	// FIX: Access fields directly on the Protobuf Cart struct, not ValidationResults
	if cart.HasConflicts {
		return nil, status.Error(codes.FailedPrecondition, "schedule conflicts detected in cart")
	}
	if len(cart.MissingPrerequisites) > 0 {
		return nil, status.Error(codes.FailedPrecondition, "prerequisites not met for some courses")
	}
	if cart.TotalUnits > shared.MaxUnitsPerSemester {
		return nil, status.Error(codes.FailedPrecondition, "max units exceeded")
	}

	// 3. Execute Transaction
	// We use the shared.WithTransaction helper
	err = shared.WithTransaction(ctx, s.client, func(sessCtx mongo.SessionContext) error {
		for _, item := range cart.Items {
			// A. Check capacity directly on DB (ensure atomic read)
			var courseDoc shared.Course
			err := s.coursesCol.FindOne(sessCtx, bson.M{"_id": item.CourseId}).Decode(&courseDoc)
			if err != nil {
				return fmt.Errorf("course %s not found during enrollment", item.CourseId)
			}

			if !courseDoc.IsOpen || courseDoc.GetSeatsAvailable() <= 0 {
				return fmt.Errorf("course %s is full or closed", item.CourseCode)
			}

			// B. Check if already enrolled
			count, _ := s.enrollmentsCol.CountDocuments(sessCtx, bson.M{
				"student_id": req.StudentId,
				"course_id":  item.CourseId,
				"status":     shared.StatusEnrolled,
			})
			if count > 0 {
				return fmt.Errorf("already enrolled in %s", item.CourseCode)
			}

			// C. Create Enrollment Record
			enrollment := shared.Enrollment{
				ID:         shared.GenerateEnrollmentID(),
				StudentID:  req.StudentId,
				CourseID:   item.CourseId,
				Status:     shared.StatusEnrolled,
				EnrolledAt: time.Now(),
				ScheduleInfo: shared.ScheduleInfo{
					Days:      item.ScheduleInfo.Days,
					StartTime: item.ScheduleInfo.StartTime,
					EndTime:   item.ScheduleInfo.EndTime,
				},
			}
			_, err = s.enrollmentsCol.InsertOne(sessCtx, enrollment)
			if err != nil {
				return err
			}

			// D. Decrement Seat
			_, err = s.coursesCol.UpdateOne(sessCtx,
				bson.M{"_id": item.CourseId},
				bson.M{"$inc": bson.M{"enrolled": 1}},
			)
			if err != nil {
				return err
			}
		}

		// E. Clear Cart on success
		_, err = s.cartsCol.DeleteOne(sessCtx, bson.M{"student_id": req.StudentId})
		return err
	})

	if err != nil {
		// Return failure
		// FIX: Access MissingPrerequisites directly for the error message
		return &pb.EnrollAllResponse{
			Success:       false,
			Message:       fmt.Sprintf("Enrollment failed: %v", err),
			FailedCourses: cart.MissingPrerequisites,
		}, nil
	}

	// 4. Retrieve newly created enrollments for response
	enrollmentsResp, _ := s.GetStudentEnrollments(ctx, &pb.GetStudentEnrollmentsRequest{
		StudentId: req.StudentId,
		Status:    shared.StatusEnrolled,
	})

	return &pb.EnrollAllResponse{
		Success:     true,
		Message:     "successfully enrolled in all courses",
		Enrollments: enrollmentsResp.Enrollments,
	}, nil
}

// DropCourse drops a student from a course
func (s *EnrollmentService) DropCourse(ctx context.Context, req *pb.DropCourseRequest) (*pb.DropCourseResponse, error) {
	if req.StudentId == "" || req.CourseId == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid args")
	}

	// Transactional Drop
	err := shared.WithTransaction(ctx, s.client, func(sessCtx mongo.SessionContext) error {
		// 1. Update Enrollment Status
		res, err := s.enrollmentsCol.UpdateOne(sessCtx,
			bson.M{
				"student_id": req.StudentId,
				"course_id":  req.CourseId,
				"status":     shared.StatusEnrolled,
			},
			bson.M{
				"$set": bson.M{
					"status":     shared.StatusDropped,
					"dropped_at": time.Now(),
				},
			},
		)
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return fmt.Errorf("enrollment not found or already dropped")
		}

		// 2. Increment Seat (Free up space)
		_, err = s.coursesCol.UpdateOne(sessCtx,
			bson.M{"_id": req.CourseId},
			bson.M{"$inc": bson.M{"enrolled": -1}},
		)
		return err
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to drop course: %v", err)
	}

	return &pb.DropCourseResponse{Success: true, Message: "course dropped"}, nil
}

// CheckConflicts checks for schedule conflicts (public RPC)
func (s *EnrollmentService) CheckConflicts(ctx context.Context, req *pb.CheckConflictsRequest) (*pb.CheckConflictsResponse, error) {
	// 1. Fetch details for all requested courses
	var cartItems []*pb.CartItem
	for _, cid := range req.CourseIds {
		cResp, err := s.courseClient.GetCourse(ctx, &pb_course.GetCourseRequest{CourseId: cid})
		if err == nil && cResp.Success {
			days, start, end := shared.ParseSchedule(cResp.Course.Schedule)
			cartItems = append(cartItems, &pb.CartItem{
				CourseId:   cResp.Course.Id,
				CourseCode: cResp.Course.Code,
				ScheduleInfo: &pb.ScheduleInfo{
					Days:      days,
					StartTime: start,
					EndTime:   end,
				},
			})
		}
	}

	// 2. Check Logic
	conflicts := s.checkScheduleConflictsInternal(cartItems)
	return &pb.CheckConflictsResponse{
		HasConflicts: len(conflicts) > 0,
		Conflicts:    conflicts,
		Message:      "conflict check complete",
	}, nil
}

// GetStudentEnrollments returns a list of enrollments
func (s *EnrollmentService) GetStudentEnrollments(ctx context.Context, req *pb.GetStudentEnrollmentsRequest) (*pb.GetStudentEnrollmentsResponse, error) {
	filter := bson.M{"student_id": req.StudentId}
	if req.Status != "" {
		filter["status"] = req.Status
	}

	cursor, err := s.enrollmentsCol.Find(ctx, filter)
	if err != nil {
		return nil, status.Error(codes.Internal, "db error")
	}
	defer cursor.Close(ctx)

	var enrollments []*pb.Enrollment
	var totalUnits int32

	for cursor.Next(ctx) {
		var doc shared.Enrollment
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		// Hydrate with Course Details
		cResp, err := s.courseClient.GetCourse(ctx, &pb_course.GetCourseRequest{CourseId: doc.CourseID})
		var code, title string
		var units int32

		if err == nil && cResp.Success {
			code = cResp.Course.Code
			title = cResp.Course.Title
			units = cResp.Course.Units
		}

		if doc.Status == shared.StatusEnrolled {
			totalUnits += units
		}

		enrollments = append(enrollments, &pb.Enrollment{
			Id:          doc.ID,
			StudentId:   doc.StudentID,
			CourseId:    doc.CourseID,
			CourseCode:  code,
			CourseTitle: title,
			Units:       units,
			Status:      doc.Status,
			EnrolledAt:  timestamppb.New(doc.EnrolledAt),
			DroppedAt:   timestamppb.New(doc.DroppedAt),
			ScheduleInfo: &pb.ScheduleInfo{
				Days:      doc.ScheduleInfo.Days,
				StartTime: doc.ScheduleInfo.StartTime,
				EndTime:   doc.ScheduleInfo.EndTime,
			},
		})
	}

	return &pb.GetStudentEnrollmentsResponse{
		Enrollments: enrollments,
		TotalUnits:  totalUnits,
	}, nil
}

// ============================================================================
// Internal Helper Functions
// ============================================================================

func (s *EnrollmentService) checkScheduleConflictsInternal(items []*pb.CartItem) []*pb.Conflict {
	var conflicts []*pb.Conflict

	// Compare every course against every other course
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			c1 := items[i]
			c2 := items[j]

			// Check Duplicate
			if c1.CourseId == c2.CourseId {
				conflicts = append(conflicts, &pb.Conflict{
					Course1Id:    c1.CourseId,
					Course1Code:  c1.CourseCode,
					Course2Id:    c2.CourseId,
					Course2Code:  c2.CourseCode,
					ConflictType: "duplicate",
					Details:      "Duplicate course selection",
				})
				continue
			}

			// Check Schedule Overlap
			// 1. Check if days overlap
			if shared.DaysOverlap(c1.ScheduleInfo.Days, c2.ScheduleInfo.Days) {
				// 2. Check if times overlap
				if shared.TimesOverlap(
					c1.ScheduleInfo.StartTime, c1.ScheduleInfo.EndTime,
					c2.ScheduleInfo.StartTime, c2.ScheduleInfo.EndTime,
				) {
					conflicts = append(conflicts, &pb.Conflict{
						Course1Id:    c1.CourseId,
						Course1Code:  c1.CourseCode,
						Course2Id:    c2.CourseId,
						Course2Code:  c2.CourseCode,
						ConflictType: "schedule",
						Details:      fmt.Sprintf("Time overlap: %s vs %s", c1.CourseCode, c2.CourseCode),
					})
				}
			}
		}
	}
	return conflicts
}
