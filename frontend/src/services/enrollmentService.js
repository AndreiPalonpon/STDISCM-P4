import api from "./api";

export const enrollmentService = {
  getCart: async (studentId) => {
    // Fixed: Path is /cart, studentId comes from token
    return api.get("/cart");
  },

  addToCart: async (studentId, courseId) => {
    // Fixed: Path is /cart/add, removed studentId from body
    return api.post("/cart/add", { course_id: courseId });
  },

  removeFromCart: async (studentId, courseId) => {
    // Fixed: Use DELETE and path parameter
    return api.delete(`/cart/remove/${courseId}`);
  },

  clearCart: async (studentId) => {
    // Fixed: Use DELETE
    return api.delete("/cart/clear");
  },

  enroll: async (studentId) => {
    // Fixed: Path is /enrollment/enroll-all
    return api.post("/enrollment/enroll-all", {});
  },

  drop: async (studentId, courseId) => {
    // Fixed: Removed student_id from body
    return api.post("/enrollment/drop", { course_id: courseId });
  },

  getEnrollments: async (studentId, filters = {}) => {
    // Fixed: Path is /enrollment/schedule, removed student_id param
    const params = new URLSearchParams();
    if (filters.semester) params.append("semester", filters.semester);
    if (filters.status) params.append("status", filters.status);

    return api.get(`/enrollment/schedule?${params}`);
  },

  // Admin/Faculty methods
  overrideEnrollment: async (studentId, courseId, action, adminId) => {
    // This connects to Admin service, path is correct based on Admin Handler
    return api.post(
      "/admin/override/" + (action === "force_enroll" ? "enroll" : "drop"),
      {
        student_id: studentId,
        course_id: courseId,
        reason: "Administrative Override",
        // admin_id is extracted from token in backend
      }
    );
  },

  // Note: There is no getClassEnrollments endpoint in the backend currently
  // You likely want gradeService.getClassRoster instead
};
