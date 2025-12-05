import api from "./api";

export const gradeService = {
  getStudentGrades: async (studentId, semester = "") => {
    // FIX: Path is /grades, studentId from token
    const params = semester ? `?semester=${semester}` : "";
    return api.get(`/grades${params}`);
  },

  calculateGPA: async (studentId, semester = "") => {
    // FIX: Path is /grades/gpa, studentId from token
    const params = semester ? `?semester=${semester}` : "";
    return api.get(`/grades/gpa${params}`);
  },

  getClassRoster: async (courseId) => {
    return api.get(`/grades/roster/${courseId}`);
  },

  uploadGrades: async (courseId, facultyId, grades) => {
    // FIX: Path includes courseId, payload structure matches 'entries'
    return api.post(`/grades/upload/${courseId}`, {
      entries: grades,
    });
  },

  publishGrades: async (courseId, facultyId) => {
    // FIX: Path includes courseId, no body needed
    return api.post(`/grades/publish/${courseId}`, {});
  },

  getCourseGrades: async (courseId, facultyId) => {
    // FIX: Removed manual faculty_id param, backend uses token
    return api.get(`/grades/course/${courseId}`);
  },
};
