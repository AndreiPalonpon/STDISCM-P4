import React from 'react';
import { BookOpen, Users, Clock, MapPin } from 'lucide-react';

const CourseCard = ({ course, actionButton, isInCart = false }) => {
  const seatsAvailable = Math.max(0, course.capacity - course.enrolled);
  const isFull = seatsAvailable === 0;

  return (
    <div className="card hover:shadow-md transition-shadow">
      <div className="p-6">
        <div className="flex justify-between items-start mb-4">
          <div>
            <h3 className="text-lg font-bold text-gray-900">{course.code}</h3>
            <h4 className="text-gray-700 font-medium">{course.title}</h4>
          </div>
          <div className={`badge ${course.is_open && !isFull ? 'badge-success' : 'badge-danger'}`}>
            {course.is_open && !isFull ? 'Open' : 'Closed'}
          </div>
        </div>

        {course.description && (
          <p className="text-gray-600 text-sm mb-4 line-clamp-2">
            {course.description}
          </p>
        )}

        <div className="space-y-3 mb-6">
          <div className="flex items-center text-sm text-gray-600">
            <BookOpen className="h-4 w-4 mr-2 flex-shrink-0" />
            <span>{course.units || 0} units</span>
          </div>
          
          <div className="flex items-center text-sm text-gray-600">
            <Users className="h-4 w-4 mr-2 flex-shrink-0" />
            <span>
              {course.enrolled || 0} / {course.capacity || 0} enrolled
              {!isFull && ` (${seatsAvailable} seats available)`}
            </span>
          </div>
          
          {course.schedule && (
            <div className="flex items-center text-sm text-gray-600">
              <Clock className="h-4 w-4 mr-2 flex-shrink-0" />
              <span>{course.schedule}</span>
            </div>
          )}
          
          {course.room && (
            <div className="flex items-center text-sm text-gray-600">
              <MapPin className="h-4 w-4 mr-2 flex-shrink-0" />
              <span>{course.room}</span>
            </div>
          )}
        </div>

        <div className="flex justify-between items-center pt-4 border-t">
          <div className="text-sm text-gray-600">
            {course.faculty_name && (
              <span>Instructor: {course.faculty_name}</span>
            )}
          </div>
          
          {actionButton && (
            <div className="ml-4">
              {actionButton}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default CourseCard;