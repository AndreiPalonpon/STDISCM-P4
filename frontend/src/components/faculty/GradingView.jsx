import React, { useState, useEffect } from 'react';
import { useAuth } from '../../hooks/useAuth';
import { gradeService } from '../../services/gradeService';
import { courseService } from '../../services/courseService';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import { Upload, Eye, Users, FileText, Check, AlertCircle } from 'lucide-react';
import { GRADE_OPTIONS } from '../../utils/constants';

const FacultyGradingView = () => {
  const { user } = useAuth();
  const [courses, setCourses] = useState([]);
  const [selectedCourse, setSelectedCourse] = useState(null);
  const [roster, setRoster] = useState([]);
  const [grades, setGrades] = useState({});
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [showGrades, setShowGrades] = useState(false);

  useEffect(() => {
    loadFacultyCourses();
  }, [user?.id]);

  useEffect(() => {
    if (selectedCourse) {
      loadClassRoster(selectedCourse.id);
    }
  }, [selectedCourse]);

  const loadFacultyCourses = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await courseService.list({ faculty_id: user?.id });
      const facultyCourses = data.courses || [];
      setCourses(facultyCourses);
      if (facultyCourses.length > 0) {
        setSelectedCourse(facultyCourses[0]);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const loadClassRoster = async (courseId) => {
    try {
      setLoading(true);
      setError(null);
      const data = await gradeService.getClassRoster(courseId);
      const rosterData = data.students || [];
      setRoster(rosterData);
      
      // Initialize grades object
      const initialGrades = {};
      rosterData.forEach(student => {
        initialGrades[student.student_id] = student.grade || '';
      });
      setGrades(initialGrades);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleGradeChange = (studentId, grade) => {
    setGrades(prev => ({
      ...prev,
      [studentId]: grade.toUpperCase()
    }));
  };

  const handleUploadGrades = async () => {
    const gradeEntries = Object.entries(grades)
      .filter(([_, grade]) => grade.trim() !== '')
      .map(([studentId, grade]) => ({
        student_id: studentId,
        grade: grade
      }));

    if (gradeEntries.length === 0) {
      setError('No grades to upload');
      return;
    }

    try {
      setUploading(true);
      setError(null);
      setSuccess(null);

      const result = await gradeService.uploadGrades(
        selectedCourse.id,
        user.id,
        gradeEntries
      );

      if (result.success) {
        setSuccess(`Successfully uploaded ${result.successful} grades`);
        if (result.failed > 0) {
          setError(`${result.failed} grades failed to upload`);
        }
      } else {
        setError('Failed to upload grades');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setUploading(false);
    }
  };

  const handlePublishGrades = async () => {
    try {
      setUploading(true);
      setError(null);
      setSuccess(null);

      const result = await gradeService.publishGrades(selectedCourse.id, user.id);

      if (result.success) {
        setSuccess(result.message || 'Grades published successfully');
        setShowGrades(true);
      } else {
        setError(result.message || 'Failed to publish grades');
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setUploading(false);
    }
  };

  if (loading) {
    return <Loader fullScreen text="Loading..." />;
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Grade Management</h1>
        <p className="text-gray-600">Upload and manage grades for your courses</p>
      </div>

      {error && (
        <Alert type="error" message={error} onClose={() => setError(null)} />
      )}

      {success && (
        <Alert type="success" message={success} onClose={() => setSuccess(null)} />
      )}

      {/* Course Selection */}
      <div className="card p-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">Select Course</h2>
            <p className="text-sm text-gray-600">Choose a course to manage grades</p>
          </div>
          <FileText className="h-6 w-6 text-gray-500" />
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {courses.map(course => (
            <button
              key={course.id}
              onClick={() => setSelectedCourse(course)}
              className={`p-4 border rounded-lg text-left transition-all ${
                selectedCourse?.id === course.id
                  ? 'border-primary-500 bg-primary-50 ring-2 ring-primary-100'
                  : 'border-gray-200 hover:border-primary-300 hover:bg-gray-50'
              }`}
            >
              <h3 className="font-bold text-gray-900">{course.code}</h3>
              <p className="text-gray-700 text-sm">{course.title}</p>
              <div className="flex items-center justify-between mt-3">
                <span className="text-sm text-gray-600">
                  {course.semester}
                </span>
                <span className="text-sm font-medium">
                  {course.enrolled} students
                </span>
              </div>
            </button>
          ))}
        </div>
      </div>

      {selectedCourse && (
        <>
          {/* Grade Management */}
          <div className="card p-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-6">
              <div>
                <h2 className="text-lg font-semibold text-gray-900">
                  {selectedCourse.code} - {selectedCourse.title}
                </h2>
                <p className="text-sm text-gray-600">
                  Manage grades for {roster.length} students
                </p>
              </div>
              
              <div className="flex space-x-3 mt-4 sm:mt-0">
                <button
                  onClick={() => setShowGrades(!showGrades)}
                  className="btn-secondary flex items-center"
                >
                  {showGrades ? (
                    <>
                      <Eye className="h-4 w-4 mr-2" />
                      Hide Grades
                    </>
                  ) : (
                    <>
                      <Eye className="h-4 w-4 mr-2" />
                      Show Grades
                    </>
                  )}
                </button>
              </div>
            </div>

            {roster.length === 0 ? (
              <div className="text-center py-8">
                <Users className="h-12 w-12 text-gray-300 mx-auto mb-4" />
                <p className="text-gray-600">No students enrolled in this course</p>
              </div>
            ) : (
              <>
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead>
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Student
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          ID
                        </th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Grade
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200">
                      {roster.map(student => (
                        <tr key={student.student_id} className="hover:bg-gray-50">
                          <td className="px-6 py-4">
                            <div className="flex items-center">
                              <div className="h-10 w-10 flex-shrink-0">
                                <div className="h-10 w-10 rounded-full bg-primary-100 flex items-center justify-center">
                                  <Users className="h-5 w-5 text-primary-600" />
                                </div>
                              </div>
                              <div className="ml-4">
                                <div className="text-sm font-medium text-gray-900">
                                  {student.student_name}
                                </div>
                                <div className="text-sm text-gray-500">
                                  {student.email}
                                </div>
                              </div>
                            </div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            <div className="text-sm text-gray-900">{student.student_id}</div>
                          </td>
                          <td className="px-6 py-4 whitespace-nowrap">
                            {showGrades ? (
                              <span className="text-sm font-medium text-gray-900">
                                {grades[student.student_id] || 'Not graded'}
                              </span>
                            ) : (
                              <select
                                value={grades[student.student_id] || ''}
                                onChange={(e) => handleGradeChange(student.student_id, e.target.value)}
                                className="input-field text-sm py-1"
                              >
                                <option value="">Select Grade</option>
                                {GRADE_OPTIONS.map(option => (
                                  <option key={option.value} value={option.value}>
                                    {option.label}
                                  </option>
                                ))}
                              </select>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Action Buttons */}
                <div className="flex flex-col sm:flex-row justify-between items-center pt-6 border-t mt-6">
                  <div className="mb-4 sm:mb-0">
                    <p className="text-sm text-gray-600">
                      {Object.values(grades).filter(g => g).length} grades entered
                    </p>
                  </div>
                  
                  <div className="flex space-x-3">
                    <button
                      onClick={handleUploadGrades}
                      disabled={uploading}
                      className="btn-primary flex items-center"
                    >
                      <Upload className="h-4 w-4 mr-2" />
                      {uploading ? 'Saving...' : 'Save Grades'}
                    </button>
                    
                    <button
                      onClick={handlePublishGrades}
                      disabled={uploading}
                      className="bg-green-600 text-white px-4 py-2 rounded-lg hover:bg-green-700 transition disabled:opacity-50 flex items-center"
                    >
                      <Check className="h-4 w-4 mr-2" />
                      Publish Grades
                    </button>
                  </div>
                </div>
              </>
            )}
          </div>

          {/* Instructions */}
          <div className="card bg-blue-50 border-blue-200 p-6">
            <h3 className="font-medium text-blue-800 mb-2 flex items-center">
              <AlertCircle className="h-5 w-5 mr-2" />
              Grading Instructions
            </h3>
            <ul className="list-disc list-inside text-blue-700 text-sm space-y-1">
              <li>Select grades from the dropdown for each student</li>
              <li>Click "Save Grades" to store grades without publishing</li>
              <li>Click "Publish Grades" to make grades visible to students</li>
              <li>Grades: A, B, C, D (passing), F (failing), I (incomplete), W (withdrawn)</li>
            </ul>
          </div>
        </>
      )}
    </div>
  );
};

export default FacultyGradingView;