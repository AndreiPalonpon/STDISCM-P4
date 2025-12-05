import React from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../hooks/useAuth';
import { useEnrollment } from '../hooks/useEnrollment';
import { useGrades } from '../hooks/useGrades';
import { useCourses } from '../hooks/useCourses';
import Alert from '../components/common/Alert';
import Loader from '../components/common/Loader.jsx';
import { 
  Home, 
  BookOpen, 
  ShoppingCart, 
  GraduationCap,
  FileText,
  Users,
  AlertCircle,
  TrendingUp,
  Calendar,
} from 'lucide-react';

const Dashboard = () => {
  const { user, isStudent, isFaculty } = useAuth();
  const { cart, enrollments, loading: enrollmentLoading } = useEnrollment();
  const { gpaInfo, loading: gradesLoading } = useGrades();
  const { courses, loading: coursesLoading } = useCourses({ open_only: true });

  const stats = [
    {
      title: 'Current GPA',
      value: gpaInfo?.cgpa?.toFixed(2) || 'N/A',
      icon: TrendingUp,
      color: 'text-green-600',
      bgColor: 'bg-green-50',
    },
    {
      title: 'Courses in Cart',
      value: cart?.courseIds?.length || 0,
      icon: ShoppingCart,
      color: 'text-blue-600',
      bgColor: 'bg-blue-50',
    },
    {
      title: 'Current Enrollments',
      value: enrollments?.filter(e => e.status === 'enrolled')?.length || 0,
      icon: BookOpen,
      color: 'text-purple-600',
      bgColor: 'bg-purple-50',
    },
    {
      title: 'Open Courses',
      value: courses?.length || 0,
      icon: Calendar,
      color: 'text-orange-600',
      bgColor: 'bg-orange-50',
    },
  ];

  if (enrollmentLoading || gradesLoading || coursesLoading) {
    return <Loader fullScreen text="Loading dashboard..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Welcome back, {user?.name}!</h1>
        <p className="text-gray-600">Here's what's happening with your account today.</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {stats.map((stat, index) => (
          <div key={index} className="card p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-gray-600">{stat.title}</p>
                <p className="text-2xl font-bold text-gray-900 mt-2">{stat.value}</p>
              </div>
              <div className={`p-3 rounded-full ${stat.bgColor}`}>
                <stat.icon className={`h-6 w-6 ${stat.color}`} />
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Student Dashboard */}
      {isStudent && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Enrollment Cart */}
          <div className="card p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-gray-900">Enrollment Cart</h2>
              <ShoppingCart className="h-5 w-5 text-gray-500" />
            </div>
            
            {cart?.courseIds?.length > 0 ? (
              <div className="space-y-4">
                <p className="text-sm text-gray-600">
                  You have {cart.courseIds.length} course(s) in your cart ready for enrollment.
                </p>
                <div className="flex space-x-3">
                  <Link to="/cart" className="btn-primary flex-1 text-center">
                    Go to Cart
                  </Link>
                </div>
              </div>
            ) : (
              <div className="text-center py-4">
                <AlertCircle className="h-12 w-12 text-gray-300 mx-auto mb-3" />
                <p className="text-gray-600">Your cart is empty</p>
                <Link to="/courses" className="mt-4 btn-secondary inline-block">
                  Browse Courses
                </Link>
              </div>
            )}
          </div>

          {/* Academic Progress */}
          <div className="card p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold text-gray-900">Academic Progress</h2>
              <GraduationCap className="h-5 w-5 text-gray-500" />
            </div>
            
            {gpaInfo ? (
              <div className="space-y-4">
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Current GPA:</span>
                  <span className="text-lg font-bold">{gpaInfo.cgpa?.toFixed(2)}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-600">Total Units:</span>
                  <span>{gpaInfo.total_units_attempted || 0}</span>
                </div>
                <Link to="/grades" className="btn-secondary w-full text-center">
                  View All Grades
                </Link>
              </div>
            ) : (
              <p className="text-gray-600">No academic records found.</p>
            )}
          </div>
        </div>
      )}

      {/* Faculty Dashboard */}
      {isFaculty && (
        <div className="card p-6">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-lg font-semibold text-gray-900">Teaching Dashboard</h2>
              <p className="text-sm text-gray-600">Manage your courses and grading</p>
            </div>
            <Users className="h-6 w-6 text-gray-500" />
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Link to="/faculty/courses" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
              <BookOpen className="h-5 w-5 text-primary-600 mb-2" />
              <h3 className="font-medium text-gray-900">My Courses</h3>
              <p className="text-sm text-gray-600 mt-1">View teaching assignments</p>
            </Link>
            
            <Link to="/faculty/grading" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
              <FileText className="h-5 w-5 text-primary-600 mb-2" />
              <h3 className="font-medium text-gray-900">Grading</h3>
              <p className="text-sm text-gray-600 mt-1">Upload and manage grades</p>
            </Link>
            
            <Link to="/courses" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
              <BookOpen className="h-5 w-5 text-primary-600 mb-2" />
              <h3 className="font-medium text-gray-900">Course Catalog</h3>
              <p className="text-sm text-gray-600 mt-1">Browse all courses</p>
            </Link>
          </div>
        </div>
      )}

      {/* Quick Links */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Quick Links</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Link to="/courses" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
            <BookOpen className="h-5 w-5 text-primary-600 mb-2" />
            <h3 className="font-medium text-gray-900">Browse Courses</h3>
            <p className="text-sm text-gray-600 mt-1">View available courses</p>
          </Link>
          
          {isStudent && (
            <>
              <Link to="/enrollments" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
                <BookOpen className="h-5 w-5 text-primary-600 mb-2" />
                <h3 className="font-medium text-gray-900">Enrollments</h3>
                <p className="text-sm text-gray-600 mt-1">Manage enrolled courses</p>
              </Link>
              
              <Link to="/grades" className="p-4 border rounded-lg hover:border-primary-300 hover:bg-primary-50 transition-colors text-left">
                <GraduationCap className="h-5 w-5 text-primary-600 mb-2" />
                <h3 className="font-medium text-gray-900">Grades</h3>
                <p className="text-sm text-gray-600 mt-1">View academic records</p>
              </Link>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;