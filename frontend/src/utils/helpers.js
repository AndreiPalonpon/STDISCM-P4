export const formatDate = (date) => {
  if (!date) return '';
  
  try {
    const d = new Date(date);
    return d.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    });
  } catch {
    return '';
  }
};

export const formatDateTime = (date) => {
  if (!date) return '';
  
  try {
    const d = new Date(date);
    return d.toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return '';
  }
};

export const calculateGPA = (grades) => {
  if (!grades || grades.length === 0) return 0;
  
  const validGrades = grades.filter(g => 
    g.grade && g.grade !== 'I' && g.grade !== 'W' && g.units
  );
  
  if (validGrades.length === 0) return 0;

  const totalPoints = validGrades.reduce((sum, grade) => {
    const gradeValue = {
      'A': 4.0, 'B': 3.0, 'C': 2.0, 'D': 1.0, 'F': 0.0,
    }[grade.grade] || 0;
    
    return sum + (gradeValue * grade.units);
  }, 0);

  const totalUnits = validGrades.reduce((sum, grade) => sum + grade.units, 0);

  return totalUnits > 0 ? (totalPoints / totalUnits) : 0;
};

export const getGradeColor = (grade) => {
  const colors = {
    A: 'text-green-700 bg-green-100',
    B: 'text-blue-700 bg-blue-100',
    C: 'text-yellow-700 bg-yellow-100',
    D: 'text-orange-700 bg-orange-100',
    F: 'text-red-700 bg-red-100',
    I: 'text-gray-700 bg-gray-100',
    W: 'text-gray-700 bg-gray-100',
  };
  
  return colors[grade] || 'text-gray-700 bg-gray-100';
};

export const truncateText = (text, maxLength = 100) => {
  if (!text || text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
};

export const debounce = (func, wait) => {
  let timeout;
  return function executedFunction(...args) {
    const later = () => {
      clearTimeout(timeout);
      func(...args);
    };
    clearTimeout(timeout);
    timeout = setTimeout(later, wait);
  };
};