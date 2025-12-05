import { useState } from "react";
import { adminService } from "../services/adminService";

export const useAdmin = () => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [successMessage, setSuccessMessage] = useState(null);

  const clearMessages = () => {
    setError(null);
    setSuccessMessage(null);
  };

  const createUser = async (userData) => {
    setLoading(true);
    clearMessages();
    try {
      await adminService.createUser(userData);
      setSuccessMessage(`Successfully created user: ${userData.email}`);
      return true;
    } catch (err) {
      setError(err.message || "Failed to create user");
      return false;
    } finally {
      setLoading(false);
    }
  };

  const createCourse = async (courseData) => {
    setLoading(true);
    clearMessages();
    try {
      await adminService.createCourse(courseData);
      setSuccessMessage(`Course created: ${courseData.code}`);
      return true;
    } catch (err) {
      setError(err.message || "Failed to create course");
      return false;
    } finally {
      setLoading(false);
    }
  };

  const deleteCourse = async (courseId) => {
    if (!window.confirm("Are you sure you want to delete this course?"))
      return false;
    setLoading(true);
    clearMessages();
    try {
      await adminService.deleteCourse(courseId);
      setSuccessMessage("Course deleted successfully");
      return true;
    } catch (err) {
      setError(err.message);
      return false;
    } finally {
      setLoading(false);
    }
  };

  const toggleSystem = async (currentState) => {
    setLoading(true);
    clearMessages();
    try {
      const newState = !currentState;
      await adminService.toggleEnrollment(newState);
      setSuccessMessage(`Enrollment is now ${newState ? "OPEN" : "CLOSED"}`);
      return newState;
    } catch (err) {
      setError(err.message);
      return currentState;
    } finally {
      setLoading(false);
    }
  };

  const performOverride = async (studentId, courseId, action, reason) => {
    setLoading(true);
    clearMessages();
    try {
      await adminService.overrideEnrollment(
        studentId,
        courseId,
        action,
        reason
      );
      setSuccessMessage(`Successfully ${action}ed student.`);
      return true;
    } catch (err) {
      setError(err.message);
      return false;
    } finally {
      setLoading(false);
    }
  };

  return {
    loading,
    error,
    successMessage,
    clearMessages,
    createUser,
    createCourse,
    deleteCourse,
    toggleSystem,
    performOverride,
  };
};
