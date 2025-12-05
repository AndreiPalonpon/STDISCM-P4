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
    // Backend handles logout via Authorization header, but we send post to trigger handler
    const response = await api.post("/auth/logout", {});
    return response;
  },

  validateToken: async (token) => {
    // Fixed: Backend expects GET request for validation
    const response = await api.get("/auth/validate");
    return response;
  },

  changePassword: async (userId, oldPassword, newPassword) => {
    // Fixed: Removed user_id from body, backend gets it from token
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
