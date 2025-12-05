import React, { useState } from 'react';
import { useGrades } from '../../hooks/useGrades';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import { GraduationCap, TrendingUp, Calendar, ChevronDown, ChevronUp } from 'lucide-react';
import { getGradeColor, formatDate } from '../../utils/helpers';

const GradesView = () => {
  const [selectedSemester, setSelectedSemester] = useState('');
  const [expandedSemesters, setExpandedSemesters] = useState({});
  const { grades, gpaInfo, loading, error } = useGrades(selectedSemester);

  const toggleSemester = (semester) => {
    setExpandedSemesters(prev => ({
      ...prev,
      [semester]: !prev[semester]
    }));
  };

  const groupGradesBySemester = () => {
    const groups = {};
    grades.forEach(grade => {
      const semester = grade.semester || 'No Semester';
      if (!groups[semester]) {
        groups[semester] = [];
      }
      groups[semester].push(grade);
    });
    return groups;
  };

  const getUniqueSemesters = () => {
    const semesters = new Set();
    grades.forEach(grade => {
      if (grade.semester) {
        semesters.add(grade.semester);
      }
    });
    return Array.from(semesters).sort().reverse();
  };

  if (loading) {
    return <Loader fullScreen text="Loading grades..." />;
  }

  if (error) {
    return (
      <Alert
        type="error"
        message={error}
      />
    );
  }

  const groupedGrades = groupGradesBySemester();
  const uniqueSemesters = getUniqueSemesters();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Academic Records</h1>
        <p className="text-gray-600">View your grades and academic progress</p>
      </div>

      {/* GPA Overview */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="card p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Current GPA</p>
              <p className="text-3xl font-bold text-gray-900 mt-2">
                {gpaInfo?.cgpa?.toFixed(2) || '0.00'}
              </p>
            </div>
            <div className="p-3 bg-primary-50 rounded-full">
              <TrendingUp className="h-6 w-6 text-primary-600" />
            </div>
          </div>
        </div>

        <div className="card p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Total Units</p>
              <p className="text-3xl font-bold text-gray-900 mt-2">
                {gpaInfo?.total_units_attempted || 0}
              </p>
            </div>
            <div className="p-3 bg-blue-50 rounded-full">
              <GraduationCap className="h-6 w-6 text-blue-600" />
            </div>
          </div>
        </div>

        <div className="card p-6">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-gray-600">Courses</p>
              <p className="text-3xl font-bold text-gray-900 mt-2">
                {grades.length}
              </p>
            </div>
            <div className="p-3 bg-green-50 rounded-full">
              <Calendar className="h-6 w-6 text-green-600" />
            </div>
          </div>
        </div>
      </div>

      {/* Semester Filters */}
      <div className="card p-6">
        <div className="flex flex-wrap items-center gap-4">
          <span className="text-sm font-medium text-gray-700">Filter by Semester:</span>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setSelectedSemester('')}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                selectedSemester === ''
                  ? 'bg-primary-100 text-primary-700'
                  : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
              }`}
            >
              All Semesters
            </button>
            {uniqueSemesters.map(semester => (
              <button
                key={semester}
                onClick={() => setSelectedSemester(semester)}
                className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                  selectedSemester === semester
                    ? 'bg-primary-100 text-primary-700'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                }`}
              >
                {semester}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Grades List */}
      <div className="space-y-4">
        {Object.entries(groupedGrades).map(([semester, semesterGrades]) => (
          <div key={semester} className="card">
            <button
              onClick={() => toggleSemester(semester)}
              className="w-full p-6 text-left"
            >
              <div className="flex justify-between items-center">
                <div>
                  <h3 className="text-lg font-semibold text-gray-900">{semester}</h3>
                  <p className="text-sm text-gray-600 mt-1">
                    {semesterGrades.length} course(s)
                  </p>
                </div>
                {expandedSemesters[semester] ? (
                  <ChevronUp className="h-5 w-5 text-gray-500" />
                ) : (
                  <ChevronDown className="h-5 w-5 text-gray-500" />
                )}
              </div>
            </button>

            {expandedSemesters[semester] && (
              <div className="px-6 pb-6">
                <div className="overflow-x-auto">
                  <table className="min-w-full divide-y divide-gray-200">
                    <thead>
                      <tr>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Course
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Title
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Units
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Grade
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Status
                        </th>
                        <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                          Date
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200">
                      {semesterGrades.map((grade) => (
                        <tr key={grade.enrollment_id}>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <div className="text-sm font-medium text-gray-900">
                              {grade.course_code}
                            </div>
                          </td>
                          <td className="px-4 py-4">
                            <div className="text-sm text-gray-900">{grade.course_title}</div>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <div className="text-sm text-gray-900">{grade.units}</div>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getGradeColor(grade.grade)}`}>
                              {grade.grade}
                            </span>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap">
                            <span className={`text-sm ${grade.published ? 'text-green-600' : 'text-yellow-600'}`}>
                              {grade.published ? 'Published' : 'Pending'}
                            </span>
                          </td>
                          <td className="px-4 py-4 whitespace-nowrap text-sm text-gray-500">
                            {formatDate(grade.published_at || grade.uploaded_at)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      {grades.length === 0 && (
        <div className="card text-center py-12">
          <GraduationCap className="h-12 w-12 text-gray-300 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No grades found</h3>
          <p className="text-gray-600">You don't have any grades recorded yet.</p>
        </div>
      )}
    </div>
  );
};

export default GradesView;