import React, { createContext, useState, useContext, useEffect } from 'react';
import { authService } from '../services/authService';
import { STORAGE_KEYS } from '../config';

// Create and export AuthContext
export const AuthContext = createContext(null);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
};

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    // Check for existing session
    const token = localStorage.getItem(STORAGE_KEYS.TOKEN);
    const userData = localStorage.getItem(STORAGE_KEYS.USER);
    
    if (token && userData) {
      try {
        setUser(JSON.parse(userData));
      } catch (err) {
        localStorage.removeItem(STORAGE_KEYS.TOKEN);
        localStorage.removeItem(STORAGE_KEYS.USER);
      }
    }
    setLoading(false);
  }, []);

  const login = async (identifier, password) => {
    try {
      setError(null);
      const response = await authService.login(identifier, password);
      
      localStorage.setItem(STORAGE_KEYS.TOKEN, response.token);
      localStorage.setItem(STORAGE_KEYS.USER, JSON.stringify(response.user));
      setUser(response.user);
      
      return response;
    } catch (err) {
      setError(err.message || 'Login failed');
      throw err;
    }
  };

  const logout = async () => {
    try {
      const token = localStorage.getItem(STORAGE_KEYS.TOKEN);
      if (token) {
        await authService.logout(token);
      }
    } catch (err) {
      console.error('Logout error:', err);
    } finally {
      localStorage.removeItem(STORAGE_KEYS.TOKEN);
      localStorage.removeItem(STORAGE_KEYS.USER);
      setUser(null);
    }
  };

  const updateUser = (userData) => {
    setUser(userData);
    localStorage.setItem(STORAGE_KEYS.USER, JSON.stringify(userData));
  };

  const value = {
    user,
    loading,
    error,
    login,
    logout,
    updateUser,
    isAuthenticated: !!user,
    isStudent: user?.role === 'student',
    isFaculty: user?.role === 'faculty',
    isAdmin: user?.role === 'admin',
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};