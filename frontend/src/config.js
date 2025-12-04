export const API_CONFIG = {
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
};

export const APP_CONFIG = {
  name: import.meta.env.VITE_APP_NAME || 'College Enrollment System',
  version: import.meta.env.VITE_APP_VERSION || '1.0.0',
};

export const STORAGE_KEYS = {
  TOKEN: 'auth_token',
  USER: 'user_data',
};

export const ROUTES = {
  LOGIN: '/login',
  DASHBOARD: '/dashboard',
  STUDENT_COURSES: '/courses',
  STUDENT_ENROLLMENTS: '/enrollments',
  STUDENT_GRADES: '/grades',
  STUDENT_CART: '/cart',
  FACULTY_COURSES: '/faculty/courses',
  FACULTY_GRADING: '/faculty/grading',
};