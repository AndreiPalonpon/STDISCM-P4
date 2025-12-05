import api from "./api";

export const enrollmentService = {
  getCart: async (studentId) => {
    // FIX: Path is /cart, studentId comes from token, unnecessary studentId in URL removed
    return api.get("/cart");
  },

  addToCart: async (studentId, courseId) => {
    // FIX: Path is /cart/add, removed studentId from body
    return api.post("/cart/add", { course_id: courseId });
  },

  removeFromCart: async (studentId, courseId) => {
    // FIX: Use DELETE and path parameter
    return api.delete(`/cart/remove/${courseId}`);
  },

  clearCart: async (studentId) => {
    // FIX: Use DELETE
    return api.delete("/cart/clear");
  },

  enroll: async (studentId) => {
    // FIX: Path is /enrollment/enroll-all
    return api.post("/enrollment/enroll-all", {});
  },

  drop: async (studentId, courseId) => {
    // FIX: Removed student_id from body
    return api.post("/enrollment/drop", { course_id: courseId });
  },

  getEnrollments: async (studentId, filters = {}) => {
    // FIX: Path is /enrollment/schedule, removed student_id param
    const params = new URLSearchParams();
    if (filters.semester) params.append("semester", filters.semester);
    if (filters.status) params.append("status", filters.status);

    return api.get(`/enrollment/schedule?${params}`);
  },

  // Admin/Faculty methods (keep these, relying on adminService now)
  overrideEnrollment: async (studentId, courseId, action, adminId) => {
    // This is handled by adminService now, but keeping for completeness if needed elsewhere
    return api.post("/enrollment/override", {
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
