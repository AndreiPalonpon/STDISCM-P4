import React, { useState } from 'react';
import { useCourses } from '../../hooks/useCourses';
import { useEnrollment } from '../../hooks/useEnrollment';
import { useAuth } from '../../hooks/useAuth';
import CourseCard from '../common/CourseCard';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import { Search, Filter, Plus, Check, X } from 'lucide-react';

const CoursesView = () => {
  const [filters, setFilters] = useState({
    search: '',
    open_only: true,
    semester: '',
  });
  const [searchInput, setSearchInput] = useState('');
  const [departments, setDepartments] = useState([]);
  const [selectedDept, setSelectedDept] = useState('');
  
  const { user } = useAuth();
  const { courses, loading, error, reload } = useCourses(filters);
  const { addToCart, removeFromCart, cart } = useEnrollment();
  const [processing, setProcessing] = useState({});

  // Extract unique departments from courses
  React.useEffect(() => {
    if (courses.length > 0) {
      const depts = [...new Set(courses
        .map(c => c.code?.split(' ')[0])
        .filter(Boolean)
      )].sort();
      setDepartments(depts);
    }
  }, [courses]);

  const handleSearch = (e) => {
    e.preventDefault();
    setFilters(prev => ({ ...prev, search: searchInput }));
  };

  const handleAddToCart = async (courseId) => {
    setProcessing(prev => ({ ...prev, [courseId]: 'adding' }));
    
    try {
      await addToCart(courseId);
    } catch (err) {
      console.error('Failed to add to cart:', err);
    } finally {
      setProcessing(prev => ({ ...prev, [courseId]: false }));
    }
  };

  const handleRemoveFromCart = async (courseId) => {
    setProcessing(prev => ({ ...prev, [courseId]: 'removing' }));
    
    try {
      await removeFromCart(courseId);
    } catch (err) {
      console.error('Failed to remove from cart:', err);
    } finally {
      setProcessing(prev => ({ ...prev, [courseId]: false }));
    }
  };

  const isInCart = (courseId) => {
    return cart?.courseIds?.includes(courseId);
  };

  const filteredCourses = courses.filter(course => {
    if (selectedDept && !course.code?.startsWith(selectedDept)) {
      return false;
    }
    return true;
  });

  if (loading) {
    return <Loader fullScreen text="Loading courses..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Course Catalog</h1>
        <p className="text-gray-600">Browse and select courses for enrollment</p>
      </div>

      {error && (
        <Alert
          type="error"
          message={error}
          onClose={() => reload()}
        />
      )}

      {/* Filters */}
      <div className="card p-6">
        <form onSubmit={handleSearch} className="space-y-4">
          <div className="flex gap-4">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 h-5 w-5" />
                <input
                  type="text"
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  placeholder="Search courses by code or title..."
                  className="input-field pl-10"
                />
              </div>
            </div>
            <button type="submit" className="btn-primary">
              Search
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <Filter className="h-5 w-5 text-gray-500" />
              <select
                value={selectedDept}
                onChange={(e) => setSelectedDept(e.target.value)}
                className="input-field"
              >
                <option value="">All Departments</option>
                {departments.map(dept => (
                  <option key={dept} value={dept}>{dept}</option>
                ))}
              </select>
            </div>

            <label className="flex items-center">
              <input
                type="checkbox"
                checked={filters.open_only}
                onChange={(e) => setFilters(prev => ({ ...prev, open_only: e.target.checked }))}
                className="h-4 w-4 text-primary-600 rounded border-gray-300"
              />
              <span className="ml-2 text-sm text-gray-700">Show only open courses</span>
            </label>
          </div>
        </form>
      </div>

      {/* Courses Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredCourses.map((course) => {
          const inCart = isInCart(course.id);
          const isProcessing = processing[course.id];
          const seatsAvailable = Math.max(0, course.capacity - course.enrolled);
          const isFull = seatsAvailable === 0;
          const isOpen = course.is_open && !isFull;

          const actionButton = isOpen ? (
            inCart ? (
              <button
                onClick={() => handleRemoveFromCart(course.id)}
                disabled={isProcessing}
                className="btn-danger flex items-center text-sm"
              >
                {isProcessing === 'removing' ? (
                  <Loader size="sm" text="" />
                ) : (
                  <>
                    <X className="h-4 w-4 mr-1" />
                    Remove
                  </>
                )}
              </button>
            ) : (
              <button
                onClick={() => handleAddToCart(course.id)}
                disabled={isProcessing || cart?.courseIds?.length >= 6}
                className="btn-primary flex items-center text-sm"
              >
                {isProcessing === 'adding' ? (
                  <Loader size="sm" text="" />
                ) : (
                  <>
                    <Plus className="h-4 w-4 mr-1" />
                    Add to Cart
                  </>
                )}
              </button>
            )
          ) : (
            <span className="text-sm text-gray-500">
              {isFull ? 'Full' : 'Closed'}
            </span>
          );

          return (
            <CourseCard
              key={course.id}
              course={course}
              actionButton={actionButton}
              isInCart={inCart}
            />
          );
        })}
      </div>

      {filteredCourses.length === 0 && !loading && (
        <div className="text-center py-12">
          <Search className="h-12 w-12 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No courses found</h3>
          <p className="text-gray-600">Try adjusting your search or filters</p>
          <button
            onClick={() => {
              setSearchInput('');
              setSelectedDept('');
              setFilters({ search: '', open_only: true, semester: '' });
            }}
            className="mt-4 btn-primary"
          >
            Clear Filters
          </button>
        </div>
      )}
    </div>
  );
};

export default CoursesView;