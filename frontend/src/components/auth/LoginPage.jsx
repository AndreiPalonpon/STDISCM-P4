import React, { useState, useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import BackendHealthCheck from '../common/BackendHealthCheck';
import { LogIn, GraduationCap, ServerCrash, WifiOff } from 'lucide-react';

const LoginPage = () => {
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [errors, setErrors] = useState({});
  const [isLoading, setIsLoading] = useState(false);
  const [backendConnected, setBackendConnected] = useState(false);
  
  const { login, user, loading } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // Redirect if already logged in
    if (user) {
      const from = location.state?.from?.pathname || '/dashboard';
      navigate(from, { replace: true });
    }
  }, [user, navigate, location]);

  const validateForm = () => {
    const newErrors = {};
    if (!identifier.trim()) newErrors.identifier = 'Email or ID is required';
    if (!password) newErrors.password = 'Password is required';
    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    
    if (!validateForm()) return;

    if (!backendConnected) {
      setErrors({ general: 'Cannot connect to backend server. Please check if the server is running.' });
      return;
    }
    
    setIsLoading(true);
    setErrors({});
    
    try {
      await login(identifier, password);
    } catch (error) {
      console.error('Login error:', error);
      
      let errorMessage = error.message || 'Login failed. Please try again.';
      let errorContent = errorMessage;

      // FIX: Enhanced Error Parsing
      if (errorMessage.toLowerCase().includes('service unavailable')) {
        errorContent = (
          <div className="flex flex-col">
            <span className="font-bold flex items-center gap-2">
              <ServerCrash size={16} /> Auth Service Unavailable
            </span>
            <span className="text-sm mt-1">
              The Gateway is working, but the <strong>Auth Microservice</strong> is not responding.
            </span>
            <span className="text-xs mt-1 opacity-75">
              Ensure the **Auth Service** node (port 50051) is running.
            </span>
          </div>
        );
      } else if (errorMessage.toLowerCase().includes('unable to connect') || errorMessage.toLowerCase().includes('fetch')) {
         errorContent = (
          <div className="flex flex-col">
             <span className="font-bold flex items-center gap-2">
              <WifiOff size={16} /> Connection Lost
            </span>
            <span className="text-sm mt-1">Unable to communicate with the server.</span>
          </div>
         );
      }

      setErrors({ general: errorContent });
    } finally {
      setIsLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Loader size="lg" text="Checking session..." />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 flex items-center justify-center p-4">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-primary-100 rounded-full mb-4">
            <GraduationCap className="w-8 h-8 text-primary-600" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 mb-2">College Enrollment System</h1>
          <p className="text-gray-600">Sign in to access your account</p>
        </div>

        {/* Backend Health Check */}
        <div className="mb-6">
          <BackendHealthCheck onStatusChange={setBackendConnected} />
        </div>

        <div className="card shadow-lg">
          <div className="p-8">
            {errors.general && (
              <Alert
                type="error"
                message={errors.general}
                className="mb-6"
                onClose={() => setErrors({})}
              />
            )}
            
            <form onSubmit={handleSubmit} className="space-y-6">
              <div>
                <label htmlFor="identifier" className="block text-sm font-medium text-gray-700 mb-1">
                  Email or Student/Faculty ID
                </label>
                <input
                  id="identifier"
                  type="text"
                  value={identifier}
                  onChange={(e) => {
                    setIdentifier(e.target.value);
                    if (errors.identifier) setErrors(prev => ({ ...prev, identifier: '' }));
                  }}
                  className="input-field"
                  placeholder="student@example.com"
                  disabled={isLoading || !backendConnected}
                />
                {errors.identifier && (
                  <p className="mt-1 text-sm text-red-600">{errors.identifier}</p>
                )}
              </div>

              <div>
                <label htmlFor="password" className="block text-sm font-medium text-gray-700 mb-1">
                  Password
                </label>
                <input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    if (errors.password) setErrors(prev => ({ ...prev, password: '' }));
                  }}
                  className="input-field"
                  placeholder="Enter your password"
                  disabled={isLoading || !backendConnected}
                />
                {errors.password && (
                  <p className="mt-1 text-sm text-red-600">{errors.password}</p>
                )}
              </div>

              <button
                type="submit"
                disabled={isLoading || !backendConnected}
                className="w-full btn-primary flex items-center justify-center py-2.5"
              >
                {isLoading ? (
                  <Loader size="sm" text="" className="mr-2" />
                ) : (
                  <LogIn className="w-5 h-5 mr-2" />
                )}
                Sign In
              </button>
            </form>

            <div className="mt-6 pt-6 border-t">
              <div className="text-center text-sm text-gray-500">
                <p className="mb-2">Demo Credentials:</p>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-2 text-xs">
                  <div className="text-left bg-gray-50 p-2 rounded border border-gray-200">
                    <p className="font-bold text-gray-700">Admin:</p>
                    <p>admin@example.com</p>
                    <p>password</p>
                  </div>
                  <div className="text-left bg-gray-50 p-2 rounded border border-gray-200">
                    <p className="font-bold text-gray-700">Student:</p>
                    <p>student@example.com</p>
                    <p>password</p>
                  </div>
                  <div className="text-left bg-gray-50 p-2 rounded border border-gray-200">
                    <p className="font-bold text-gray-700">Faculty:</p>
                    <p>faculty@example.com</p>
                    <p>password</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-8 text-center text-sm text-gray-500">
          <p>© {new Date().getFullYear()} College Enrollment System</p>
          <p className="mt-1">Distributed Systems Project - Fault Tolerant Architecture</p>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;