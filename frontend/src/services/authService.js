import api from "./api";

export const authService = {
  login: async (identifier, password) => {
    const response = await api.post("/auth/login", { identifier, password });

    if (!response.success) {
      throw new Error(response.message || "Login failed");
    }

    return response;
  },

  logout: async (token) => {
    // FIX: Logout uses Authorization header, body is typically empty or optional
    const response = await api.post("/auth/logout", {});
    return response;
  },

  validateToken: async (token) => {
    // FIX: Backend expects GET request for validation, token is in Header
    const response = await api.get("/auth/validate");
    return response;
  },

  changePassword: async (userId, oldPassword, newPassword) => {
    // FIX: Removed user_id from body, backend gets it from token
    const response = await api.post("/auth/change-password", {
      old_password: oldPassword,
      new_password: newPassword,
    });

    if (!response.success) {
      throw new Error(response.message || "Password change failed");
    }

    return response;
  },
};
