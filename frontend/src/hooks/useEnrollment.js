import { useState, useEffect, useCallback } from "react";
import { enrollmentService } from "../services/enrollmentService";
import { useAuth } from "./useAuth";

export const useEnrollment = () => {
  const { user } = useAuth();
  const [cart, setCart] = useState(null);
  const [enrollments, setEnrollments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // FIX: Memoize loadCart using useCallback
  const loadCart = useCallback(async () => {
    if (!user?.id) return;

    try {
      const data = await enrollmentService.getCart(user.id);
      setCart(data.cart);
    } catch (err) {
      // Handle "cart not found" or empty cart gracefully
      if (err.message.includes("not found") || err.message.includes("empty")) {
        setCart({ courseIds: [], total_units: 0 }); // Set to an empty but valid cart object
      } else {
        setError(err.message);
      }
    }
  }, [user?.id]);

  // FIX: Memoize loadEnrollments using useCallback
  const loadEnrollments = useCallback(async () => {
    if (!user?.id) return;

    try {
      setLoading(true);
      const data = await enrollmentService.getEnrollments(user.id);
      setEnrollments(data.enrollments || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [user?.id]);

  const addToCart = useCallback(
    async (courseId) => {
      if (!user?.id) return;
      try {
        const response = await enrollmentService.addToCart(user.id, courseId);
        setCart(response.cart);
        return true;
      } catch (err) {
        setError(err.message);
        throw err;
      }
    },
    [user?.id]
  );

  const removeFromCart = useCallback(
    async (courseId) => {
      if (!user?.id) return;
      try {
        const response = await enrollmentService.removeFromCart(
          user.id,
          courseId
        );
        setCart(response.cart);
        return true;
      } catch (err) {
        setError(err.message);
        throw err;
      }
    },
    [user?.id]
  );

  const enroll = useCallback(async () => {
    if (!user?.id) return;
    try {
      const response = await enrollmentService.enroll(user.id);
      loadCart();
      loadEnrollments();
      return response;
    } catch (err) {
      setError(err.message);
      throw err;
    }
  }, [user?.id, loadCart, loadEnrollments]);

  const dropCourse = useCallback(
    async (courseId) => {
      if (!user?.id) return;
      try {
        await enrollmentService.drop(user.id, courseId);
        loadEnrollments();
        return true;
      } catch (err) {
        setError(err.message);
        throw err;
      }
    },
    [user?.id, loadEnrollments]
  );

  // FIX: Main useEffect runs with stable dependencies
  useEffect(() => {
    if (user?.id) {
      loadCart();
      loadEnrollments();
    }
  }, [user?.id, loadCart, loadEnrollments]);

  return {
    cart,
    enrollments,
    loading,
    error,
    addToCart,
    removeFromCart,
    enroll,
    dropCourse,
    reload: () => {
      loadCart();
      loadEnrollments();
    },
  };
};
