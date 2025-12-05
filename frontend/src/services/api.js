import { API_CONFIG } from '../config';

class ApiService {
  constructor() {
    this.baseURL = API_CONFIG.baseURL;
    this.timeout = API_CONFIG.timeout;
  }

  async request(endpoint, options = {}) {
    const token = localStorage.getItem('auth_token');
    const headers = {
      ...API_CONFIG.headers,
      ...(token && { 'Authorization': `Bearer ${token}` }),
      ...options.headers,
    };

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const url = `${this.baseURL}${endpoint}`;
      console.log('API Request:', { url, method: options.method || 'GET' });

      const response = await fetch(url, {
        ...options,
        headers,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      // Handle 401 Unauthorized
      if (response.status === 401) {
        localStorage.removeItem('auth_token');
        localStorage.removeItem('user_data');
        if (window.location.pathname !== '/login') {
          window.location.href = '/login';
        }
        throw new Error('Session expired. Please login again.');
      }

      // Parse response
      const contentType = response.headers.get('content-type');
      let data;
      
      if (contentType && contentType.includes('application/json')) {
        data = await response.json();
      } else {
        const text = await response.text();
        console.error('Non-JSON response:', text);
        throw new Error('Server returned invalid response format');
      }

      // Handle error responses
      if (!response.ok) {
        const errorMessage = data.message || data.error || `Request failed with status ${response.status}`;
        throw new Error(errorMessage);
      }

      // Check for success flag in response
      if (data.hasOwnProperty('success') && !data.success) {
        throw new Error(data.message || 'Request failed');
      }

      return data;
    } catch (error) {
      clearTimeout(timeoutId);
      
      console.error('API Error:', {
        endpoint,
        error: error.message,
        stack: error.stack
      });

      // Handle specific error types
      if (error.name === 'AbortError') {
        throw new Error('Request timeout. The server is taking too long to respond.');
      }
      
      if (error.message === 'Failed to fetch') {
        throw new Error('Unable to connect to the server. Please check:\n1. Backend server is running on port 8080\n2. Network connection is stable\n3. CORS is properly configured');
      }

      if (error instanceof TypeError && error.message.includes('NetworkError')) {
        throw new Error('Network error. Please verify the backend server is running.');
      }

      // Re-throw the error with context
      throw error;
    }
  }

  get(endpoint) {
    return this.request(endpoint, { method: 'GET' });
  }

  post(endpoint, data) {
    return this.request(endpoint, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  put(endpoint, data) {
    return this.request(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  patch(endpoint, data) {
    return this.request(endpoint, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  delete(endpoint, data) {
    return this.request(endpoint, {
      method: 'DELETE',
      ...(data && { body: JSON.stringify(data) }),
    });
  }
}

export default new ApiService();