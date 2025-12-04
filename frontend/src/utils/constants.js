export const ROLES = {
  STUDENT: 'student',
  FACULTY: 'faculty',
  ADMIN: 'admin',
};

export const ENROLLMENT_STATUS = {
  ENROLLED: 'enrolled',
  DROPPED: 'dropped',
  COMPLETED: 'completed',
};

export const GRADE_VALUES = {
  A: 4.0,
  B: 3.0,
  C: 2.0,
  D: 1.0,
  F: 0.0,
  I: 0.0,
  W: 0.0,
};

export const GRADE_OPTIONS = [
  { value: 'A', label: 'A - Excellent' },
  { value: 'B', label: 'B - Good' },
  { value: 'C', label: 'C - Satisfactory' },
  { value: 'D', label: 'D - Passing' },
  { value: 'F', label: 'F - Failing' },
  { value: 'I', label: 'I - Incomplete' },
  { value: 'W', label: 'W - Withdrawn' },
];

export const MAX_CART_COURSES = 6;
export const MAX_UNITS_PER_SEMESTER = 18;