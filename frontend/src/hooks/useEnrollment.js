import { useState, useEffect, useCallback } from 'react';
import { enrollmentService } from '../services/enrollmentService';
import { useAuth } from './useAuth';

export const useEnrollment = () => {
  const { user } = useAuth();
  const [cart, setCart] = useState(null);
  const [enrollments, setEnrollments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const loadCart = useCallback(async () => {
    if (!user?.id) return;
    
    try {
      const data = await enrollmentService.getCart(user.id);
      setCart(data.cart);
    } catch (err) {
      setError(err.message);
    }
  }, [user?.id]);

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

  const addToCart = useCallback(async (courseId) => {
    if (!user?.id) return;
    
    try {
      await enrollmentService.addToCart(user.id, courseId);
      await loadCart();
      return true;
    } catch (err) {
      setError(err.message);
      throw err;
    }
  }, [user?.id, loadCart]);

  const removeFromCart = useCallback(async (courseId) => {
    if (!user?.id) return;
    
    try {
      await enrollmentService.removeFromCart(user.id, courseId);
      await loadCart();
      return true;
    } catch (err) {
      setError(err.message);
      throw err;
    }
  }, [user?.id, loadCart]);

  const enroll = useCallback(async () => {
    if (!user?.id) return;
    
    try {
      const response = await enrollmentService.enroll(user.id);
      await loadCart();
      await loadEnrollments();
      return response;
    } catch (err) {
      setError(err.message);
      throw err;
    }
  }, [user?.id, loadCart, loadEnrollments]);

  const dropCourse = useCallback(async (courseId) => {
    if (!user?.id) return;
    
    try {
      await enrollmentService.drop(user.id, courseId);
      await loadEnrollments();
      return true;
    } catch (err) {
      setError(err.message);
      throw err;
    }
  }, [user?.id, loadEnrollments]);

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
