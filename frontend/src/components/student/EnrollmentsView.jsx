import React, { useState } from 'react';
import { useEnrollment } from '../../hooks/useEnrollment';
import { useCourses } from '../../hooks/useCourses';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import { BookOpen, Clock, MapPin, Users, Trash2, CheckCircle } from 'lucide-react';

const EnrollmentsView = () => {
  const { enrollments, dropCourse, loading, error } = useEnrollment();
  const { courses } = useCourses({});
  const [dropping, setDropping] = useState(null);

  const handleDrop = async (courseId) => {
    if (!window.confirm('Are you sure you want to drop this course?')) {
      return;
    }

    setDropping(courseId);
    try {
      await dropCourse(courseId);
    } catch (err) {
      console.error('Failed to drop course:', err);
    } finally {
      setDropping(null);
    }
  };

  const currentEnrollments = enrollments.filter(e => e.status === 'enrolled');

  if (loading) {
    return <Loader fullScreen text="Loading enrollments..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Current Enrollments</h1>
        <p className="text-gray-600">Manage your currently enrolled courses</p>
      </div>

      {error && (
        <Alert type="error" message={error} />
      )}

      {currentEnrollments.length === 0 ? (
        <div className="card text-center py-12">
          <BookOpen className="h-12 w-12 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No current enrollments</h3>
          <p className="text-gray-600">You are not currently enrolled in any courses</p>
          <button
            onClick={() => window.location.href = '/courses'}
            className="mt-4 btn-primary"
          >
            Browse Courses
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {currentEnrollments.map((enrollment) => {
            const course = courses.find(c => c.id === enrollment.courseId);
            
            if (!course) return null;

            return (
              <div key={enrollment.id} className="card p-6">
                <div className="flex justify-between items-start mb-4">
                  <div>
                    <h3 className="text-lg font-bold text-gray-900">{course.code}</h3>
                    <h4 className="text-gray-700 font-medium">{course.title}</h4>
                    <div className="flex items-center mt-2 text-sm text-green-600">
                      <CheckCircle className="h-4 w-4 mr-1" />
                      <span>Enrolled</span>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDrop(course.id)}
                    disabled={dropping === course.id}
                    className="btn-danger flex items-center text-sm"
                  >
                    {dropping === course.id ? (
                      <Loader size="sm" text="" />
                    ) : (
                      <>
                        <Trash2 className="h-4 w-4 mr-1" />
                        Drop Course
                      </>
                    )}
                  </button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mt-4">
                  <div className="flex items-center text-sm text-gray-600">
                    <BookOpen className="h-4 w-4 mr-2" />
                    <span>{course.units} units</span>
                  </div>
                  
                  <div className="flex items-center text-sm text-gray-600">
                    <Users className="h-4 w-4 mr-2" />
                    <span>{course.enrolled} / {course.capacity} students</span>
                  </div>
                  
                  {course.schedule && (
                    <div className="flex items-center text-sm text-gray-600">
                      <Clock className="h-4 w-4 mr-2" />
                      <span>{course.schedule}</span>
                    </div>
                  )}
                  
                  {course.room && (
                    <div className="flex items-center text-sm text-gray-600">
                      <MapPin className="h-4 w-4 mr-2" />
                      <span>{course.room}</span>
                    </div>
                  )}
                </div>

                {course.faculty_name && (
                  <div className="mt-4 pt-4 border-t">
                    <p className="text-sm text-gray-600">
                      Instructor: <span className="font-medium">{course.faculty_name}</span>
                    </p>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default EnrollmentsView;