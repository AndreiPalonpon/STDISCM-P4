import React from 'react';
import { useAuth } from '../hooks/useAuth';
import Navigation from '../components/common/Navigation';

// Import Admin component and helper components
import AdminDashboard from '../components/admin/AdminDashboard'; 
import CoursesView from '../components/student/CoursesView';
import EnrollmentsView from '../components/student/EnrollmentsView';
import StudentGradesView from '../components/student/GradesView';
import ShoppingCart from '../components/student/ShoppingCart';
import FacultyCoursesView from '../components/faculty/CoursesView';
import ClassRoster from '../components/faculty/ClassRoster';
import GradingView from '../components/faculty/GradingView';

const Dashboard = () => {
  const { user } = useAuth();
  // State to manage internal navigation for non-admin roles
  const [currentView, setCurrentView] = React.useState('dashboard');

  const renderContent = () => {
    // FIX: Render Admin Dashboard if user is admin
    if (user?.role === 'admin') {
      return <AdminDashboard />;
    }

    // --- FACULTY VIEWS ---
    if (user?.role === 'faculty') {
      // NOTE: In a complete implementation, this switch would use React Router, 
      // but sticking to the existing pattern for now.
      switch (currentView) {
        case 'roster':
          return <ClassRoster />;
        case 'grading':
          return <GradingView />;
        default:
          return <FacultyCoursesView />;
      }
    }

    // --- STUDENT VIEWS ---
    switch (currentView) {
      case 'enrollments':
        return <EnrollmentsView />;
      case 'grades':
        return <StudentGradesView />;
      case 'cart':
        return <ShoppingCart />;
      case 'courses':
      default:
        return <CoursesView />;
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Navigation component handles rendering based on user role */}
      <Navigation />
      
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {renderContent()}
      </main>
    </div>
  );
};

export default Dashboard;