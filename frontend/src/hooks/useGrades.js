import { useState, useEffect, useCallback } from 'react';
import { gradeService } from '../services/gradeService';
import { useAuth } from './useAuth';

export const useGrades = (semester = '') => {
  const { user } = useAuth();
  const [grades, setGrades] = useState([]);
  const [gpaInfo, setGpaInfo] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const loadGrades = useCallback(async () => {
    if (!user?.id) return;
    
    try {
      setLoading(true);
      setError(null);
      const data = await gradeService.getStudentGrades(user.id, semester);
      setGrades(data.grades || []);
      setGpaInfo(data.gpa_info || null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [user?.id, semester]);

  useEffect(() => {
    if (user?.id) {
      loadGrades();
    }
  }, [user?.id, loadGrades]);

  return {
    grades,
    gpaInfo,
    loading,
    error,
    reload: loadGrades,
  };
};