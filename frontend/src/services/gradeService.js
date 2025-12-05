import api from "./api";

export const gradeService = {
  getStudentGrades: async (studentId, semester = "") => {
    // Fixed: Path is /grades, studentId from token
    const params = semester ? `?semester=${semester}` : "";
    return api.get(`/grades${params}`);
  },

  calculateGPA: async (studentId, semester = "") => {
    // Fixed: Path is /grades/gpa, studentId from token
    const params = semester ? `?semester=${semester}` : "";
    return api.get(`/grades/gpa${params}`);
  },

  getClassRoster: async (courseId) => {
    return api.get(`/grades/roster/${courseId}`);
  },

  uploadGrades: async (courseId, facultyId, grades) => {
    // Fixed: Path includes courseId, payload structure matches 'entries'
    // Input 'grades' is expected to be array of { student_id, grade }
    return api.post(`/grades/upload/${courseId}`, {
      entries: grades,
    });
  },

  publishGrades: async (courseId, facultyId) => {
    // Fixed: Path includes courseId, no body needed
    return api.post(`/grades/publish/${courseId}`, {});
  },

  getCourseGrades: async (courseId, facultyId) => {
    // Fixed: Removed manual faculty_id param
    return api.get(`/grades/course/${courseId}`);
  },
};
