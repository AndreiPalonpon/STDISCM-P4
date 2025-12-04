import api from './api';

export const courseService = {
  list: async (filters = {}) => {
    const params = new URLSearchParams();
    if (filters.department) params.append('department', filters.department);
    if (filters.search) params.append('search', filters.search);
    if (filters.open_only) params.append('open_only', 'true');
    if (filters.semester) params.append('semester', filters.semester);
    
    const query = params.toString() ? `?${params}` : '';
    return api.get(`/courses${query}`);
  },

  get: async (id) => {
    return api.get(`/courses/${id}`);
  },

  getAvailability: async (id) => {
    return api.get(`/courses/${id}/availability`);
  },

  checkPrerequisites: async (courseId, studentId) => {
    return api.get(`/courses/${courseId}/prerequisites?student_id=${studentId}`);
  },

  // Faculty methods
  create: async (courseData) => {
    return api.post('/courses', courseData);
  },

  update: async (id, courseData) => {
    return api.put(`/courses/${id}`, courseData);
  },

  delete: async (id) => {
    return api.delete(`/courses/${id}`);
  },
};