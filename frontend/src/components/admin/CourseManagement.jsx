import React, { useState, useEffect } from 'react';
import { useAdmin } from '../../hooks/useAdmin';
import { courseService } from '../../services/courseService';
import { Plus, Trash2, Edit } from 'lucide-react';
import Loader from '../common/Loader';

const CourseManagement = () => {
  const { createCourse, deleteCourse, loading: adminLoading } = useAdmin();
  const [courses, setCourses] = useState([]);
  const [loadingCourses, setLoadingCourses] = useState(false);
  const [newCourse, setNewCourse] = useState({ 
    code: '', title: '', units: 3, capacity: 30, room: '', schedule: '' 
  });

  useEffect(() => {
    loadCourses();
  }, []);

  const loadCourses = async () => {
    setLoadingCourses(true);
    try {
      const res = await courseService.list();
      setCourses(res.courses || []); // NOTE: Accessing courses property directly
    } catch (e) { 
      console.error("Failed to load courses", e); 
      setCourses([]);
    } finally {
      setLoadingCourses(false);
    }
  };

  const handleAdd = async (e) => {
    e.preventDefault();
    const success = await createCourse(newCourse);
    if (success) {
      setNewCourse({ code: '', title: '', units: 3, capacity: 30, room: '', schedule: '' });
      loadCourses();
    }
  };

  const handleDelete = async (id) => {
    const success = await deleteCourse(id);
    if (success) loadCourses();
  };

  const isActionLoading = adminLoading || loadingCourses;

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div className="lg:col-span-1">
        <div className="bg-gray-50 p-4 rounded-lg border">
          <h3 className="font-bold mb-4 flex items-center gap-2"><Plus size={16}/> New Course</h3>
          <form onSubmit={handleAdd} className="space-y-3">
            <input type="text" placeholder="Course Code (CS101)" className="input-field" required 
              value={newCourse.code} onChange={e => setNewCourse({...newCourse, code: e.target.value})} 
              disabled={isActionLoading} />
            <input type="text" placeholder="Title" className="input-field" required 
              value={newCourse.title} onChange={e => setNewCourse({...newCourse, title: e.target.value})} 
              disabled={isActionLoading} />
            <div className="grid grid-cols-2 gap-2">
              <input type="number" placeholder="Units" className="input-field" required 
                value={newCourse.units} onChange={e => setNewCourse({...newCourse, units: parseInt(e.target.value)})} 
                disabled={isActionLoading} />
              <input type="number" placeholder="Cap" className="input-field" required 
                value={newCourse.capacity} onChange={e => setNewCourse({...newCourse, capacity: parseInt(e.target.value)})} 
                disabled={isActionLoading} />
            </div>
            <input type="text" placeholder="Room" className="input-field" required 
              value={newCourse.room} onChange={e => setNewCourse({...newCourse, room: e.target.value})} 
              disabled={isActionLoading} />
            <input type="text" placeholder="Schedule" className="input-field" required 
              value={newCourse.schedule} onChange={e => setNewCourse({...newCourse, schedule: e.target.value})} 
              disabled={isActionLoading} />
            <button type="submit" className="btn-primary w-full" disabled={isActionLoading}>
              {adminLoading ? <Loader size="sm" text="Adding..." /> : 'Add Course'}
            </button>
          </form>
        </div>
      </div>

      <div className="lg:col-span-2">
        <h3 className="font-bold mb-4">Existing Courses ({courses.length})</h3>
        <div className="bg-white border rounded-lg overflow-hidden">
          {loadingCourses && (
            <div className="p-4 text-center">
              <Loader size="sm" text="Loading courses..." />
            </div>
          )}
          {!loadingCourses && courses.length > 0 && (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Code</th>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Title</th>
                    <th className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Cap</th>
                    <th className="px-4 py-2 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200">
                  {courses.map(c => (
                    <tr key={c.id}>
                      <td className="px-4 py-3 text-sm font-bold text-gray-900">{c.code}</td>
                      <td className="px-4 py-3 text-sm text-gray-500">{c.title}</td>
                      <td className="px-4 py-3 text-sm text-gray-500">{c.enrolled || 0}/{c.capacity}</td>
                      <td className="px-4 py-3 text-right">
                        <button onClick={() => alert('Editing is not yet implemented.')} className="text-indigo-600 hover:text-indigo-900 mr-4" disabled={isActionLoading}><Edit size={16} /></button>
                        <button onClick={() => handleDelete(c.id)} className="text-red-600 hover:text-red-900" disabled={isActionLoading}>
                          <Trash2 size={16} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {!loadingCourses && courses.length === 0 && <p className="p-4 text-center text-gray-500">No courses found. Add one above.</p>}
        </div>
      </div>
    </div>
  );
};

export default CourseManagement;