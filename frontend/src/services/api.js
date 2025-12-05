import { API_CONFIG } from "../config";
import { STORAGE_KEYS } from "../config";

class ApiService {
  constructor() {
    this.baseURL = API_CONFIG.baseURL;
    this.timeout = API_CONFIG.timeout;
  }

  async request(endpoint, options = {}) {
    const token = localStorage.getItem(STORAGE_KEYS.TOKEN);
    const headers = {
      ...API_CONFIG.headers,
      ...(token && { Authorization: `Bearer ${token}` }),
      ...options.headers,
    };

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const url = `${this.baseURL}${endpoint}`;

      const response = await fetch(url, {
        ...options,
        headers,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      // Parse response
      const contentType = response.headers.get("content-type");
      let data;

      if (contentType && contentType.includes("application/json")) {
        data = await response.json();
      } else {
        const text = await response.text();
        if (!response.ok) {
          throw new Error(text || "Server error");
        }
        data = { success: true, message: text };
      }

      // FIX: Differentiate 401 during login vs. session expiration
      if (response.status === 401) {
        // If we are currently trying to login, a 401 means "Wrong Password/User",
        if (endpoint.includes("/auth/login")) {
          throw new Error(data.message || "Invalid credentials");
        }

        // For all other endpoints, 401 means the token is bad/expired.
        localStorage.removeItem(STORAGE_KEYS.TOKEN);
        localStorage.removeItem(STORAGE_KEYS.USER);

        if (!window.location.pathname.includes("/login")) {
          window.location.href = "/login";
        }
        throw new Error("Session expired. Please login again.");
      }

      // Handle other error responses
      if (!response.ok) {
        const errorMessage =
          data.message ||
          data.error ||
          `Request failed with status ${response.status}`;
        throw new Error(errorMessage);
      }

      // Check for success flag in response
      if (data.hasOwnProperty("success") && !data.success) {
        throw new Error(data.message || "Request failed");
      }

      return data;
    } catch (error) {
      clearTimeout(timeoutId);

      console.error("API Error:", {
        endpoint,
        error: error.message,
      });

      if (error.name === "AbortError") {
        throw new Error(
          "Request timeout. The server is taking too long to respond."
        );
      }

      if (error.message.includes("Failed to fetch")) {
        throw new Error(
          "Unable to connect to the server. Please check:\n1. Backend server is running on port 8080\n2. Network connection is stable"
        );
      }

      throw error;
    }
  }

  get(endpoint) {
    return this.request(endpoint, { method: "GET" });
  }

  post(endpoint, data) {
    return this.request(endpoint, {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  put(endpoint, data) {
    return this.request(endpoint, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  patch(endpoint, data) {
    return this.request(endpoint, {
      method: "PATCH",
      body: JSON.stringify(data),
    });
  }

  delete(endpoint, data) {
    return this.request(endpoint, {
      method: "DELETE",
      ...(data && { body: JSON.stringify(data) }),
    });
  }
}

export default new ApiService();
