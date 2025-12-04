import React, { useState, useEffect } from 'react';
import { useCourses } from '../../hooks/useCourses';
import { useAuth } from '../../hooks/useAuth';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import CourseCard from '../common/CourseCard';
import { BookOpen, Users, Filter } from 'lucide-react';

const FacultyCoursesView = () => {
  const { user } = useAuth();
  const [filters, setFilters] = useState({});
  const [selectedSemester, setSelectedSemester] = useState('');
  const [semesters, setSemesters] = useState([]);
  
  const { courses, loading, error } = useCourses(filters);

  useEffect(() => {
    // Extract unique semesters
    if (courses.length > 0) {
      const uniqueSemesters = [...new Set(courses.map(c => c.semester).filter(Boolean))].sort();
      setSemesters(uniqueSemesters);
      if (uniqueSemesters.length > 0 && !selectedSemester) {
        setSelectedSemester(uniqueSemesters[0]);
      }
    }
  }, [courses]);

  useEffect(() => {
    if (user?.id) {
      setFilters(prev => ({ 
        ...prev, 
        faculty_id: user.id,
        semester: selectedSemester 
      }));
    }
  }, [user?.id, selectedSemester]);

  if (loading) {
    return <Loader fullScreen text="Loading your courses..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">My Courses</h1>
        <p className="text-gray-600">Manage the courses you are teaching</p>
      </div>

      {error && (
        <Alert type="error" message={error} />
      )}

      {/* Semester Filter */}
      {semesters.length > 0 && (
        <div className="card p-6">
          <div className="flex items-center gap-4">
            <Filter className="h-5 w-5 text-gray-500" />
            <div className="flex flex-wrap gap-2">
              {semesters.map(semester => (
                <button
                  key={semester}
                  onClick={() => setSelectedSemester(semester)}
                  className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                    selectedSemester === semester
                      ? 'bg-primary-100 text-primary-700'
                      : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                  }`}
                >
                  {semester}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Courses Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {courses.map((course) => (
          <div key={course.id} className="card p-6 hover:shadow-md transition-shadow">
            <div className="flex justify-between items-start mb-4">
              <div>
                <h3 className="text-lg font-bold text-gray-900">{course.code}</h3>
                <h4 className="text-gray-700 font-medium">{course.title}</h4>
                <div className="mt-2 text-sm text-gray-600">
                  {course.semester}
                </div>
              </div>
              <div className={`badge ${course.is_open ? 'badge-success' : 'badge-danger'}`}>
                {course.is_open ? 'Open' : 'Closed'}
              </div>
            </div>

            <div className="space-y-3 mb-6">
              <div className="flex items-center text-sm text-gray-600">
                <BookOpen className="h-4 w-4 mr-2" />
                <span>{course.units} units</span>
              </div>
              
              <div className="flex items-center text-sm text-gray-600">
                <Users className="h-4 w-4 mr-2" />
                <span>{course.enrolled} students enrolled</span>
              </div>
              
              {course.schedule && (
                <div className="flex items-center text-sm text-gray-600">
                  <span className="ml-6">{course.schedule}</span>
                </div>
              )}
              
              {course.room && (
                <div className="flex items-center text-sm text-gray-600">
                  <span className="ml-6">{course.room}</span>
                </div>
              )}
            </div>

            <div className="pt-4 border-t">
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-600">
                  Seats available: {Math.max(0, course.capacity - course.enrolled)}
                </span>
                <button
                  onClick={() => window.location.href = `/faculty/roster/${course.id}`}
                  className="btn-primary text-sm"
                >
                  View Roster
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {courses.length === 0 && !loading && (
        <div className="card text-center py-12">
          <BookOpen className="h-12 w-12 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No teaching assignments</h3>
          <p className="text-gray-600">You are not assigned to teach any courses for the selected semester.</p>
        </div>
      )}
    </div>
  );
};

export default FacultyCoursesView;