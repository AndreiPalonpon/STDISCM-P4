// frontend/src/components/student/ShoppingCart.jsx
import React, { useState } from 'react';
import { useEnrollment } from '../../hooks/useEnrollment';
import { useCourses } from '../../hooks/useCourses';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import CourseCard from '../common/CourseCard';
import { CheckCircle, Trash2, AlertTriangle, ShoppingCart as CartIcon } from 'lucide-react';

const ShoppingCart = () => {
  const { cart, enroll, removeFromCart, loading, error } = useEnrollment();
  const { courses } = useCourses({});
  const [enrolling, setEnrolling] = useState(false);
  const [removing, setRemoving] = useState(null);

  const cartCourses = courses.filter(course => 
    cart?.courseIds?.includes(course.id)
  );

  const calculateTotalUnits = () => {
    return cartCourses.reduce((total, course) => total + (course.units || 0), 0);
  };

  const handleEnroll = async () => {
    if (!window.confirm('Are you sure you want to enroll in all courses in your cart?')) {
      return;
    }

    setEnrolling(true);
    try {
      const result = await enroll();
      if (result.success) {
        window.location.href = '/enrollments';
      }
    } catch (err) {
      console.error('Enrollment failed:', err);
    } finally {
      setEnrolling(false);
    }
  };

  const handleRemove = async (courseId) => {
    setRemoving(courseId);
    try {
      await removeFromCart(courseId);
    } catch (err) {
      console.error('Failed to remove from cart:', err);
    } finally {
      setRemoving(null);
    }
  };

  if (loading) {
    return <Loader fullScreen text="Loading cart..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Shopping Cart</h1>
        <p className="text-gray-600">Review and enroll in your selected courses</p>
      </div>

      {error && (
        <Alert type="error" message={error} />
      )}

      {cartCourses.length === 0 ? (
        <div className="card text-center py-12">
          <ShoppingCart className="h-12 w-12 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">Your cart is empty</h3>
          <p className="text-gray-600">Add courses from the course catalog to get started.</p>
          <button
            onClick={() => window.location.href = '/courses'}
            className="mt-4 btn-primary"
          >
            Browse Courses
          </button>
        </div>
      ) : (
        <>
          <div className="space-y-4">
            {cartCourses.map((course) => {
              const seatsAvailable = Math.max(0, course.capacity - course.enrolled);
              const isFull = seatsAvailable === 0;
              const isOpen = course.is_open && !isFull;

              const actionButton = (
                <button
                  onClick={() => handleRemove(course.id)}
                  disabled={removing === course.id}
                  className="btn-danger flex items-center text-sm"
                >
                  {removing === course.id ? (
                    <Loader size="sm" text="" />
                  ) : (
                    <>
                      <Trash2 className="h-4 w-4 mr-1" />
                      Remove
                    </>
                  )}
                </button>
              );

              return (
                <CourseCard
                  key={course.id}
                  course={course}
                  actionButton={actionButton}
                />
              );
            })}
          </div>

          {/* Cart Summary */}
          <div className="card p-6">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold text-gray-900">Cart Summary</h3>
                <div className="mt-2 space-y-1">
                  <p className="text-sm text-gray-600">
                    {cartCourses.length} course(s) selected
                  </p>
                  <p className="text-sm text-gray-600">
                    Total units: {calculateTotalUnits()}
                  </p>
                  {calculateTotalUnits() > 18 && (
                    <p className="text-sm text-yellow-600 flex items-center">
                      <AlertTriangle className="h-4 w-4 mr-1" />
                      Exceeds maximum units per semester (18)
                    </p>
                  )}
                </div>
              </div>

              <div className="flex flex-col sm:flex-row gap-3">
                <button
                  onClick={() => window.location.href = '/courses'}
                  className="btn-secondary"
                >
                  Add More Courses
                </button>
                <button
                  onClick={handleEnroll}
                  disabled={enrolling || calculateTotalUnits() > 18}
                  className="btn-primary flex items-center justify-center"
                >
                  {enrolling ? (
                    <Loader size="sm" text="" />
                  ) : (
                    <>
                      <CheckCircle className="h-5 w-5 mr-2" />
                      Enroll All
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>

          {/* Validation Warnings */}
          {cart?.validation_results?.has_conflicts && (
            <Alert
              type="warning"
              title="Schedule Conflicts Detected"
              message="Some courses in your cart have scheduling conflicts. Please review your selections."
            />
          )}

          {cart?.validation_results?.missing_prerequisites?.length > 0 && (
            <Alert
              type="warning"
              title="Missing Prerequisites"
              message={`You are missing prerequisites for: ${cart.validation_results.missing_prerequisites.join(', ')}`}
            />
          )}
        </>
      )}
    </div>
  );
};

export default ShoppingCart;