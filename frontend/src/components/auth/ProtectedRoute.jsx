import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import Loader from '../common/Loader';

const ProtectedRoute = ({ children, role }) => {
  const { user, loading, isAuthenticated } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader size="lg" text="Loading..." />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (role) {
    const userRole = user?.role;
    
    if (role === 'student' && userRole !== 'student') {
      return <Navigate to="/dashboard" replace />;
    }
    
    if (role === 'faculty' && userRole !== 'faculty') {
      return <Navigate to="/dashboard" replace />;
    }
    
    if (role === 'admin' && userRole !== 'admin') {
      return <Navigate to="/dashboard" replace />;
    }
  }

  return children;
};

export default ProtectedRoute;