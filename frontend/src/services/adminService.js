import api from "./api";

export const adminService = {
  // --- User Management ---
  createUser: async (userData) => {
    // userData: { name, email, role, password, student_id?, faculty_id?, department? }
    return api.post("/admin/users", userData);
  },

  getAllUsers: async () => {
    return api.get("/admin/users");
  },

  // --- Course Management ---
  createCourse: async (courseData) => {
    return api.post("/admin/courses", courseData);
  },

  updateCourse: async (id, courseData) => {
    return api.put(`/admin/courses/${id}`, courseData);
  },

  deleteCourse: async (id) => {
    return api.delete(`/admin/courses/${id}`);
  },

  assignFaculty: async (courseId, facultyId) => {
    return api.post(`/admin/courses/${courseId}/assign-faculty`, {
      faculty_id: facultyId,
    });
  },

  // --- System Controls ---
  setEnrollmentPeriod: async (startDate, endDate) => {
    return api.post("/admin/enrollment/period", {
      start_date: startDate,
      end_date: endDate,
    });
  },

  toggleEnrollment: async (enable) => {
    return api.post("/admin/enrollment/toggle", { enable });
  },

  // Override: Force enroll/drop specific students
  overrideEnrollment: async (studentId, courseId, action, reason) => {
    const endpoint =
      action === "enroll" ? "/admin/override/enroll" : "/admin/override/drop";
    return api.post(endpoint, {
      student_id: studentId,
      course_id: courseId,
      reason: reason || "Administrative Override",
    });
  },
};
