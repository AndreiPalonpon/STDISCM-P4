import React, { useState } from 'react';
import { useAdmin } from '../../hooks/useAdmin';
import Alert from '../common/Alert';
import Loader from '../common/Loader';
import UserManagement from './UserManagement';
import CourseManagement from './CourseManagement';
import SystemControls from './SystemControls';
import { Users, BookOpen, Settings, LayoutDashboard } from 'lucide-react';

const AdminDashboard = () => {
  const [activeTab, setActiveTab] = useState('overview');
  const { error, successMessage, clearMessages, loading } = useAdmin();

  const renderTabContent = () => {
    switch (activeTab) {
      case 'users': return <UserManagement />;
      case 'courses': return <CourseManagement />;
      case 'system': return <SystemControls />;
      default: return <OverviewTab setActiveTab={setActiveTab} />;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Admin Dashboard</h1>
          <p className="text-gray-500">Manage system resources and configurations</p>
        </div>
      </div>

      {/* Global Alerts */}
      {(error || successMessage) && (
        <Alert 
          type={error ? 'error' : 'success'} 
          message={error || successMessage} 
          onClose={clearMessages} 
        />
      )}

      {/* Navigation Tabs */}
      <div className="bg-white shadow rounded-lg p-2">
        <nav className="flex space-x-2 overflow-x-auto">
          <TabButton id="overview" label="Overview" icon={LayoutDashboard} active={activeTab} onClick={setActiveTab} />
          <TabButton id="users" label="User Management" icon={Users} active={activeTab} onClick={setActiveTab} />
          <TabButton id="courses" label="Course Management" icon={BookOpen} active={activeTab} onClick={setActiveTab} />
          <TabButton id="system" label="System Controls" icon={Settings} active={activeTab} onClick={setActiveTab} />
        </nav>
      </div>

      {/* Content Area */}
      <div className="bg-white shadow rounded-lg p-6 min-h-[500px]">
        {loading && <div className="mb-4"><Loader size="sm" /></div>}
        {renderTabContent()}
      </div>
    </div>
  );
};

const TabButton = ({ id, label, icon: Icon, active, onClick }) => (
  <button
    onClick={() => onClick(id)}
    className={`flex items-center px-4 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap
      ${active === id 
        ? 'bg-primary-50 text-primary-700' 
        : 'text-gray-500 hover:bg-gray-50 hover:text-gray-700'
      }`}
  >
    <Icon className="w-4 h-4 mr-2" />
    {label}
  </button>
);

const OverviewTab = ({ setActiveTab }) => (
  <div className="text-center py-12">
    <div className="bg-primary-50 inline-flex p-4 rounded-full mb-4">
      <Settings className="w-12 h-12 text-primary-600" />
    </div>
    <h3 className="text-xl font-medium text-gray-900 mb-2">Welcome to the Admin Portal</h3>
    <p className="text-gray-500 max-w-lg mx-auto mb-8">
      Select a module from the tabs above to begin managing the enrollment system.
    </p>
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 max-w-4xl mx-auto">
      <QuickAction title="Add Users" desc="Create student/faculty accounts" onClick={() => setActiveTab('users')} />
      <QuickAction title="Manage Courses" desc="Add or edit course offerings" onClick={() => setActiveTab('courses')} />
      <QuickAction title="System Status" desc="Open/Close enrollment period" onClick={() => setActiveTab('system')} />
    </div>
  </div>
);

const QuickAction = ({ title, desc, onClick }) => (
  <button onClick={onClick} className="p-4 border rounded-lg hover:border-primary-500 hover:bg-primary-50 transition text-left">
    <h4 className="font-bold text-gray-900">{title}</h4>
    <p className="text-sm text-gray-500">{desc}</p>
  </button>
);

export default AdminDashboard;