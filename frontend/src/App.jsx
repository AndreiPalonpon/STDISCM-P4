import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import Navigation from './components/common/Navigation';
import ProtectedRoute from './components/auth/ProtectedRoute';
import LoginPage from './components/auth/LoginPage';
import StudentCoursesView from './components/student/CoursesView';
import StudentEnrollmentsView from './components/student/EnrollmentsView';
import StudentGradesView from './components/student/GradesView';
import ShoppingCart from './components/student/ShoppingCart';
import FacultyCoursesView from './components/faculty/CoursesView';
import FacultyGradingView from './components/faculty/GradingView';
import ClassRoster from './components/faculty/ClassRoster';
import Dashboard from './pages/Dashboard';

function App() {
  return (
    <Router>
      <AuthProvider>
        <div className="min-h-screen bg-gray-50">
          <Navigation />
          <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route path="/" element={<Navigate to="/login" replace />} />
              
              <Route path="/dashboard" element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              } />
              
              {/* Student Routes */}
              <Route path="/courses" element={
                <ProtectedRoute role="student">
                  <StudentCoursesView />
                </ProtectedRoute>
              } />
              
              <Route path="/enrollments" element={
                <ProtectedRoute role="student">
                  <StudentEnrollmentsView />
                </ProtectedRoute>
              } />
              
              <Route path="/grades" element={
                <ProtectedRoute role="student">
                  <StudentGradesView />
                </ProtectedRoute>
              } />
              
              <Route path="/cart" element={
                <ProtectedRoute role="student">
                  <ShoppingCart />
                </ProtectedRoute>
              } />
              
              {/* Faculty Routes */}
              <Route path="/faculty/courses" element={
                <ProtectedRoute role="faculty">
                  <FacultyCoursesView />
                </ProtectedRoute>
              } />
              
              <Route path="/faculty/grading" element={
                <ProtectedRoute role="faculty">
                  <FacultyGradingView />
                </ProtectedRoute>
              } />
              
              <Route path="/faculty/roster/:courseId" element={
                <ProtectedRoute role="faculty">
                  <ClassRoster />
                </ProtectedRoute>
              } />
              
              {/* Admin Routes (can be added later) */}
              
              <Route path="*" element={
                <div className="text-center py-12">
                  <h1 className="text-2xl font-bold text-gray-900">404 - Page Not Found</h1>
                  <p className="mt-2 text-gray-600">The page you're looking for doesn't exist.</p>
                </div>
              } />
            </Routes>
          </main>
        </div>
      </AuthProvider>
    </Router>
  );
}

export default App;