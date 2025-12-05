import React, { useState } from 'react';
import { useAdmin } from '../../hooks/useAdmin';
import { ShieldAlert, ToggleLeft, ToggleRight, Loader } from 'lucide-react';

const SystemControls = () => {
  const { toggleSystem, performOverride } = useAdmin();
  const [enabled, setEnabled] = useState(false);
  const [override, setOverride] = useState({ studentId: '', courseId: '', reason: '' });
  const [overrideLoading, setOverrideLoading] = useState(false);
  const [toggleLoading, setToggleLoading] = useState(false);

  const handleToggle = async () => {
    setToggleLoading(true);
    const newState = await toggleSystem(enabled);
    setEnabled(newState);
    setToggleLoading(false);
  };

  const handleOverride = async (action) => {
    if (!override.studentId || !override.courseId) return;
    setOverrideLoading(true);
    const success = await performOverride(override.studentId, override.courseId, action, override.reason);
    if (success) {
      setOverride({ studentId: '', courseId: '', reason: '' });
    }
    setOverrideLoading(false);
  };

  const isSystemBusy = toggleLoading || overrideLoading;

  return (
    <div className="space-y-8 max-w-3xl mx-auto">
      {/* Enrollment Toggle */}
      <div className="bg-white p-6 rounded-lg border border-gray-200 shadow-sm flex items-center justify-between">
        <div>
          <h3 className="text-lg font-bold text-gray-900">Global Enrollment Status</h3>
          <p className="text-sm text-gray-500">Controls whether students can access the enrollment pages.</p>
        </div>
        <button 
          onClick={handleToggle}
          className={`flex items-center gap-2 px-6 py-3 rounded-full font-bold text-white transition-colors
            ${enabled ? 'bg-red-500 hover:bg-red-600' : 'bg-green-500 hover:bg-green-700'}
            ${isSystemBusy ? 'opacity-50 cursor-not-allowed' : ''}`}
          disabled={isSystemBusy}
        >
          {toggleLoading ? <Loader size="sm" text="Wait..." /> : (
            <>
              {enabled ? <ToggleRight size={24} /> : <ToggleLeft size={24} />}
              {enabled ? 'STOP ENROLLMENT' : 'START ENROLLMENT'}
            </>
          )}
        </button>
      </div>

      {/* Manual Override */}
      <div className="bg-red-50 p-6 rounded-lg border border-red-100">
        <h3 className="text-lg font-bold text-red-900 flex items-center gap-2 mb-4">
          <ShieldAlert size={20} /> Administrative Override
        </h3>
        <p className="text-sm text-red-700 mb-4">
          Forcefully enroll or drop a student regardless of prerequisites or capacity restrictions.
        </p>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <input 
            type="text" placeholder="Student ID" className="input-field"
            value={override.studentId} onChange={e => setOverride({...override, studentId: e.target.value})}
            disabled={overrideLoading}
          />
          <input 
            type="text" placeholder="Course Code" className="input-field"
            value={override.courseId} onChange={e => setOverride({...override, courseId: e.target.value})}
            disabled={overrideLoading}
          />
          <input 
            type="text" placeholder="Reason for override" className="input-field md:col-span-2"
            value={override.reason} onChange={e => setOverride({...override, reason: e.target.value})}
            disabled={overrideLoading}
          />
        </div>

        <div className="flex gap-4 mt-4">
          <button 
            onClick={() => handleOverride('enroll')}
            className="flex-1 bg-green-600 text-white py-2 rounded hover:bg-green-700 font-medium"
            disabled={overrideLoading || !override.studentId || !override.courseId}
          >
            {overrideLoading && <Loader size="sm" text="Processing..." />}
            {!overrideLoading && 'Force Enroll'}
          </button>
          <button 
            onClick={() => handleOverride('drop')}
            className="flex-1 bg-red-600 text-white py-2 rounded hover:bg-red-700 font-medium"
            disabled={overrideLoading || !override.studentId || !override.courseId}
          >
            {overrideLoading && <Loader size="sm" text="Processing..." />}
            {!overrideLoading && 'Force Drop'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default SystemControls;