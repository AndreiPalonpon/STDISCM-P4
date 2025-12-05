# College Enrollment System - Frontend

A distributed, fault-tolerant web application for college course enrollment built with React.

## Features

### Student Features
- Browse and search available courses
- Add courses to shopping cart
- Enroll in multiple courses
- View enrollments and drop courses
- View grades and GPA

### Faculty Features
- View assigned courses
- Enter and upload grades
- Publish grades to students
- View class rosters

## Tech Stack

- **React 18** - UI Framework
- **Vite** - Build tool
- **Tailwind CSS** - Styling
- **Lucide React** - Icons
- **Axios** - HTTP client

## Setup Instructions

### Prerequisites
- Node.js 18+ and npm/yarn
- Backend services running (Gateway on port 8080)

### Installation

1. Clone the repository and navigate to frontend directory:
```bash
cd frontend
```

2. Install dependencies:
```bash
npm install
```

3. Create environment file:
```bash
cp .env.example .env
```

4. Update `.env` with your API endpoint:
```
VITE_API_BASE_URL=http://localhost:8080/api
```

5. Start development server:
```bash
npm run dev
```

6. Access the application:
```
http://localhost:3000
```

### Build for Production

```bash
npm run build
```

The build artifacts will be in the `dist/` directory.

## Project Structure

```
src/
├── components/       # Reusable UI components
│   ├── common/      # Shared components
│   ├── auth/        # Authentication components
│   ├── student/     # Student-specific components
│   └── faculty/     # Faculty-specific components
├── contexts/        # React contexts (Auth, etc.)
├── services/        # API service layers
├── hooks/           # Custom React hooks
├── utils/           # Helper functions
├── styles/          # Global styles
└── App.jsx          # Main app component
```

## Distributed System Features

### Fault Tolerance
- Graceful degradation when services are unavailable
- Clear error messages for service failures
- Session persistence across page refreshes
- Retry logic for failed requests

### Service Independence
- Auth Service: Login/logout/session management
- Course Service: Course catalog and information
- Enrollment Service: Course enrollment operations
- Grade Service: Grade entry and viewing
- Admin Service: System administration

When a service is down, only features dependent on that service are affected.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| VITE_API_BASE_URL | Gateway API endpoint | http://localhost:8080/api |
| VITE_APP_NAME | Application name | College Enrollment System |
| VITE_APP_VERSION | Application version | 1.0.0 |

## Demo Credentials

### Student Account
- Email: `student@example.com`
- Password: `password`

### Faculty Account
- Email: `faculty@example.com`
- Password: `password`

## Development

### Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

### Code Style
- Use functional components with hooks
- Follow React best practices
- Use Tailwind utility classes for styling
- Keep components small and focused
- Extract reusable logic into custom hooks

## API Integration

The frontend communicates with the backend through a RESTful API gateway.

### Authentication
- JWT tokens stored in localStorage
- Token sent in Authorization header
- Automatic logout on 401 responses

### Error Handling
- Network errors show "Service unavailable"
- API errors display specific error messages
- Loading states during API calls

## Deployment

### Using Docker

```bash
docker build -t enrollment-frontend .
docker run -p 3000:80 enrollment-frontend
```

### Using Nginx

1. Build the application:
```bash
npm run build
```

2. Copy `dist/` contents to nginx web root:
```bash
cp -r dist/* /var/www/html/
```

3. Configure nginx to serve the SPA:
```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

## Troubleshooting

### API Connection Issues
- Verify backend gateway is running on port 8080
- Check CORS configuration on gateway
- Ensure `.env` has correct API URL

### Build Failures
- Clear node_modules and reinstall: `rm -rf node_modules && npm install`
- Clear Vite cache: `rm -rf .vite`

### Authentication Issues
- Clear browser localStorage
- Check JWT token expiration
- Verify auth service is running

## Contributing

1. Follow the existing code structure
2. Add tests for new features
3. Update documentation
4. Follow conventional commits

## License

MIT License - See LICENSE file for details