import React, { useState } from 'react';
import { useAdmin } from '../../hooks/useAdmin';
import { UserPlus } from 'lucide-react';
import Loader from '../common/Loader';

const UserManagement = () => {
  const { createUser, loading } = useAdmin();
  const [formData, setFormData] = useState({
    name: '', email: '', password: 'password', role: 'student',
    student_id: '', faculty_id: '', department: ''
  });

  const handleSubmit = async (e) => {
    e.preventDefault();
    const success = await createUser(formData);
    if (success) {
      setFormData({
        ...formData,
        name: '', email: '', 
        student_id: formData.role === 'student' ? '' : formData.student_id,
        faculty_id: formData.role === 'faculty' ? '' : formData.faculty_id,
        department: formData.role === 'faculty' ? '' : formData.department,
        password: 'password'
      });
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <div className="mb-6 pb-6 border-b">
        <h2 className="text-lg font-bold flex items-center gap-2">
          <UserPlus className="w-5 h-5" /> Create New Account
        </h2>
        <p className="text-sm text-gray-500">Add a new Student, Faculty, or Admin to the system.</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <input 
            type="text" placeholder="Full Name" className="input-field" required
            value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})}
            disabled={loading}
          />
          <select 
            className="input-field" value={formData.role} 
            onChange={e => setFormData({...formData, role: e.target.value})}
            disabled={loading}
          >
            <option value="student">Student</option>
            <option value="faculty">Faculty</option>
            <option value="admin">Admin</option>
          </select>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <input 
            type="email" placeholder="Email Address" className="input-field" required
            value={formData.email} onChange={e => setFormData({...formData, email: e.target.value})}
            disabled={loading}
          />
          <input 
            type="password" placeholder="Initial Password" className="input-field" required
            value={formData.password} onChange={e => setFormData({...formData, password: e.target.value})}
            disabled={loading}
          />
        </div>

        {/* Dynamic Fields based on Role */}
        {formData.role === 'student' && (
          <div className="p-4 bg-gray-50 rounded-md border border-gray-200">
            <h3 className="text-xs font-bold text-gray-500 uppercase mb-2">Student Details</h3>
            <input 
              type="text" placeholder="Student ID (e.g. 2024001)" className="input-field" required
              value={formData.student_id} onChange={e => setFormData({...formData, student_id: e.target.value})}
              disabled={loading}
            />
          </div>
        )}

        {formData.role === 'faculty' && (
          <div className="p-4 bg-gray-50 rounded-md border border-gray-200 grid grid-cols-2 gap-4">
            <div className="col-span-2">
               <h3 className="text-xs font-bold text-gray-500 uppercase mb-2">Faculty Details</h3>
            </div>
            <input 
              type="text" placeholder="Faculty ID (e.g. FAC-001)" className="input-field" required
              value={formData.faculty_id} onChange={e => setFormData({...formData, faculty_id: e.target.value})}
              disabled={loading}
            />
            <input 
              type="text" placeholder="Department" className="input-field" required
              value={formData.department} onChange={e => setFormData({...formData, department: e.target.value})}
              disabled={loading}
            />
          </div>
        )}

        <button type="submit" className="btn-primary w-full" disabled={loading}>
          {loading ? <Loader size="sm" text="Creating..." /> : 'Create Account'}
        </button>
      </form>
    </div>
  );
};

export default UserManagement;