import api from './api';

export const gradeService = {
  getStudentGrades: async (studentId, semester = '') => {
    const params = semester ? `?semester=${semester}` : '';
    return api.get(`/grades/student/${studentId}${params}`);
  },

  calculateGPA: async (studentId, semester = '') => {
    const params = semester ? `?semester=${semester}` : '';
    return api.get(`/grades/gpa/${studentId}${params}`);
  },

  getClassRoster: async (courseId) => {
    return api.get(`/grades/roster/${courseId}`);
  },

  uploadGrades: async (courseId, facultyId, grades) => {
    return api.post('/grades/upload', {
      course_id: courseId,
      faculty_id: facultyId,
      grades: grades,
    });
  },

  publishGrades: async (courseId, facultyId) => {
    return api.post('/grades/publish', {
      course_id: courseId,
      faculty_id: facultyId,
    });
  },

  getCourseGrades: async (courseId, facultyId) => {
    const params = facultyId ? `?faculty_id=${facultyId}` : '';
    return api.get(`/grades/course/${courseId}${params}`);
  },
};