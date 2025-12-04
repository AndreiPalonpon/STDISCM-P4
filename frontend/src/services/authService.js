import api from './api';

export const authService = {
  login: async (identifier, password) => {
    const response = await api.post('/auth/login', { identifier, password });
    
    if (!response.success) {
      throw new Error(response.message || 'Login failed');
    }
    
    return response;
  },

  logout: async (token) => {
    const response = await api.post('/auth/logout', { token });
    return response;
  },

  validateToken: async (token) => {
    const response = await api.post('/auth/validate', { token });
    return response;
  },

  changePassword: async (userId, oldPassword, newPassword) => {
    const response = await api.post('/auth/change-password', {
      user_id: userId,
      old_password: oldPassword,
      new_password: newPassword,
    });
    
    if (!response.success) {
      throw new Error(response.message || 'Password change failed');
    }
    
    return response;
  },
};