import api from './api';

export const enrollmentService = {
  getCart: async (studentId) => {
    return api.get(`/enrollment/cart/${studentId}`);
  },

  addToCart: async (studentId, courseId) => {
    return api.post('/enrollment/cart/add', { student_id: studentId, course_id: courseId });
  },

  removeFromCart: async (studentId, courseId) => {
    return api.post('/enrollment/cart/remove', { student_id: studentId, course_id: courseId });
  },

  clearCart: async (studentId) => {
    return api.post('/enrollment/cart/clear', { student_id: studentId });
  },

  enroll: async (studentId) => {
    return api.post('/enrollment/enroll', { student_id: studentId });
  },

  drop: async (studentId, courseId) => {
    return api.post('/enrollment/drop', { student_id: studentId, course_id: courseId });
  },

  getEnrollments: async (studentId, filters = {}) => {
    const params = new URLSearchParams({ student_id: studentId });
    if (filters.semester) params.append('semester', filters.semester);
    if (filters.status) params.append('status', filters.status);
    
    return api.get(`/enrollment/student?${params}`);
  },

  // Admin/Faculty methods
  overrideEnrollment: async (studentId, courseId, action, adminId) => {
    return api.post('/enrollment/override', {
      student_id: studentId,
      course_id: courseId,
      action: action,
      admin_id: adminId,
    });
  },

  getClassEnrollments: async (courseId) => {
    return api.get(`/enrollment/course/${courseId}`);
  },
};