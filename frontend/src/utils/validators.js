export const validateEmail = (email) => {
  const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return re.test(email);
};

export const validatePassword = (password) => {
  return password && password.length >= 6;
};

export const validateGrade = (grade) => {
  const validGrades = ['A', 'B', 'C', 'D', 'F', 'I', 'W'];
  return validGrades.includes(grade.toUpperCase());
};

export const validateForm = (fields, values) => {
  const errors = {};
  
  fields.forEach(field => {
    const value = values[field.name];
    
    if (field.required && !value) {
      errors[field.name] = `${field.label} is required`;
      return;
    }
    
    if (field.type === 'email' && value && !validateEmail(value)) {
      errors[field.name] = 'Please enter a valid email address';
    }
    
    if (field.type === 'password' && value && !validatePassword(value)) {
      errors[field.name] = 'Password must be at least 6 characters';
    }
    
    if (field.minLength && value && value.length < field.minLength) {
      errors[field.name] = `${field.label} must be at least ${field.minLength} characters`;
    }
    
    if (field.maxLength && value && value.length > field.maxLength) {
      errors[field.name] = `${field.label} must be at most ${field.maxLength} characters`;
    }
  });
  
  return errors;
};