// frontend/src/components/common/BackendHealthCheck.jsx

import React, { useState, useEffect } from 'react';
import Alert from './Alert';
import { AlertCircle, CheckCircle, RefreshCw } from 'lucide-react';

/**
 * BackendHealthCheck Component
 * 
 * Monitors the connection to the backend server and provides
 * visual feedback and troubleshooting information.
 * 
 * Props:
 * - onStatusChange: (isConnected: boolean) => void
 *   Callback function that receives connection status updates
 */
const BackendHealthCheck = ({ onStatusChange }) => {
  const [status, setStatus] = useState('checking'); // 'checking' | 'connected' | 'error'
  const [error, setError] = useState(null);
  const [retrying, setRetrying] = useState(false);

  /**
   * Checks if the backend server is responding
   * by making a simple GET request to the courses endpoint
   */
  const checkBackend = async () => {
    try {
      setStatus('checking');
      setError(null);
      
      // Create abort controller for timeout
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000); // 5 second timeout

      // Try to fetch from a public endpoint
      const response = await fetch('http://localhost:8080/api/courses?open_only=true', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      // If response is OK, backend is running
      if (response.ok) {
        setStatus('connected');
        onStatusChange?.(true);
      } else {
        throw new Error(`Server responded with status ${response.status}`);
      }
    } catch (err) {
      console.error('Backend health check failed:', err);
      setStatus('error');
      
      // Provide specific error messages based on error type
      if (err.name === 'AbortError') {
        setError('Connection timeout. Server is not responding.');
      } else if (err.message === 'Failed to fetch') {
        setError('Cannot connect to backend server. Please ensure:\n• Backend is running on port 8080\n• Run: cd backend && ./scripts/start-all.ps1');
      } else {
        setError(err.message);
      }
      
      onStatusChange?.(false);
    }
  };

  /**
   * Manual retry handler
   */
  const handleRetry = async () => {
    setRetrying(true);
    await checkBackend();
    setRetrying(false);
  };

  /**
   * Check on mount and set up periodic checks
   */
  useEffect(() => {
    checkBackend();
    
    // Check every 30 seconds
    const interval = setInterval(checkBackend, 30000);
    
    return () => clearInterval(interval);
  }, []);

  // Render different UI based on status
  if (status === 'checking') {
    return (
      <div className="flex items-center justify-center p-4 bg-blue-50 rounded-lg">
        <RefreshCw className="animate-spin h-5 w-5 text-blue-600 mr-2" />
        <span className="text-blue-800">Connecting to backend...</span>
      </div>
    );
  }

  if (status === 'connected') {
    return (
      <div className="flex items-center p-3 bg-green-50 rounded-lg border border-green-200">
        <CheckCircle className="h-5 w-5 text-green-600 mr-2 flex-shrink-0" />
        <span className="text-green-800 text-sm font-medium">Connected to backend server</span>
      </div>
    );
  }

  if (status === 'error') {
    return (
      <Alert
        type="error"
        title="Backend Connection Failed"
        message={
          <div className="space-y-3">
            <p className="whitespace-pre-line">{error}</p>
            
            {/* Instructions box */}
            <div className="bg-red-50 p-3 rounded border border-red-200">
              <p className="text-sm font-medium mb-2">To start the backend:</p>
              <div className="space-y-2">
                <pre className="text-xs bg-red-900 text-white p-2 rounded overflow-x-auto">
cd backend{'\n'}
./scripts/start-all.ps1
                </pre>
                <p className="text-xs text-red-700">
                  Wait 10-15 seconds for all services to start, then click retry.
                </p>
              </div>
            </div>
            
            {/* Retry button */}
            <button
              onClick={handleRetry}
              disabled={retrying}
              className="btn-primary w-full flex items-center justify-center"
            >
              {retrying ? (
                <>
                  <RefreshCw className="animate-spin h-4 w-4 mr-2" />
                  Retrying...
                </>
              ) : (
                <>
                  <RefreshCw className="h-4 w-4 mr-2" />
                  Retry Connection
                </>
              )}
            </button>
          </div>
        }
      />
    );
  }

  return null;
};

export default BackendHealthCheck;