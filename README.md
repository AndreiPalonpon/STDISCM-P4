# Distributed College Enrollment System

A fault-tolerant, microservices-based college enrollment system built with a React frontend and a Golang backend. This project demonstrates a distributed architecture using a Gateway pattern to manage communication between the client and multiple specialized backend services.

## 📖 Overview

The system is designed to handle the core academic processes of a university: course scheduling, student enrollment, and grade management. It creates a robust distributed system where each core function runs as an isolated service node.

### Architecture

The application follows a **Microservices Architecture** with a central **API Gateway**:

```
               +-----------------------------+
               |       FRONTEND (React)      |
               +--------------+--------------+
                              | (HTTP/REST)
                              v
+-------------------------------------------------------------+
|                    API GATEWAY (Go)                         |
| (Routes: REST <> gRPC, Fault Tolerance, Auth Check)         |
+-----------+-----------+-----------+-----------+-------------+
    | gRPC      | gRPC      | gRPC      | gRPC      | gRPC
    v (50051)   v (50052)   v (50053)   v (50054)   v (50055)
+---------+ +---------+ +---------+ +---------+ +---------+
|  AUTH   | | COURSE  | | ENROLL  | |  GRADE  | |  ADMIN  |
| Service | | Service | | Service | | Service | | Service |
+----+----+ +----+----+ +----+----+ +----+----+ +----+----+
     |           |           |           |           |
     |           |           |           |           |
     |           |           |           |           |
     +-----+-----+-----+-----+-----+-----+-----+-----+
                              | (MongoDB Driver)
                              v
                       +-------------+
                       |   MongoDB   |
                       +-------------+
```

- **Frontend Node**: A Single Page Application (SPA) built with React and Vite that communicates exclusively with the Gateway via REST.
- **Gateway Node**: A Go HTTP server (Middleman) that handles routing, protocol translation (REST to gRPC), and load balancing.
- **Service Nodes**: Independent Go gRPC servers for specific domains:
  - **Auth Service**: User authentication and session management.
  - **Course Service**: Course catalog and prerequisite checking.
  - **Enrollment Service**: Cart management and enrollment transactions.
  - **Grade Service**: Grading, GPA calculation, and roster management.
  - **Admin Service**: System configuration and user/course management.
- **Database**: MongoDB is used for data persistence, with collections logically separated by service.

## ✨ Features

### Student

- **Course Discovery**: Browse and search for available courses by department or title.
- **Enrollment**: Add courses to a shopping cart, validate prerequisites, and enroll in bulk.
- **Schedule Management**: View current class schedule and drop courses.
- **Academic Records**: View grades and automatically calculated GPA (Term and Cumulative).

### Faculty

- **Course Management**: View assigned teaching loads.
- **Class Rosters**: Access lists of enrolled students for specific courses.
- **Grading**: Upload grades via CSV or enter them manually, with a publishing workflow.

### Admin

- **User Management**: Create and manage accounts for students and faculty.
- **Course Catalog**: Create, update, and delete courses; assign faculty.
- **System Controls**: Set enrollment periods and toggle system-wide enrollment status.
- **Overrides**: Force-enroll or drop students to resolve conflicts.

## 🛠️ Tech Stack

- **Frontend**: React 18, Vite, Tailwind CSS, Axios, Lucide React.
- **Backend**: Go (Golang) 1.22+, Chi Router (Gateway), gRPC/Protobuf (Internal Communication).
- **Database**: MongoDB.
- **Tools**: PowerShell scripts for orchestration.

## 📂 Project Structure

```
college-enrollment-system/
├── backend/
│   ├── cmd/                 # Entry points for all services
│   │   ├── admin/           # Admin Service
│   │   ├── auth/            # Auth Service
│   │   ├── course/          # Course Service
│   │   ├── enrollment/      # Enrollment Service
│   │   ├── gateway/         # HTTP API Gateway
│   │   └── grade/           # Grade Service
│   ├── internal/            # Private application logic and gRPC handlers
│   ├── pb/                  # Compiled Protocol Buffer files
│   ├── protos/              # Source .proto definitions
│   └── shared/              # Shared utilities (DB config, models)
├── frontend/
│   ├── src/
│   │   ├── components/      # React components (Auth, Student, Faculty)
│   │   ├── contexts/        # Global state (AuthContext)
│   │   ├── services/        # API service adapters
│   │   └── hooks/           # Custom React hooks
├── scripts/                 # Start/Stop automation scripts
└── README.md
```

## Getting Started

### Prerequisites

- **Go:** Version 1.22 or higher.

- **Node.js:** Version 18 or higher.

- **MongoDB:** A running instance (local or Atlas) on port 27017 or a valid connection string.

### Installation

1. **Clone the repository:**

   ```
   git clone https://github.com/AndreiPalonpon/STDISCM-P4.git
   cd stdiscm-p4
   ```

2. **Install Frontend Dependencies:**

   ```
   cd frontend
   npm install
   cd ..
   ```

3. **Setup Environment Variables:**
   - Ensure the backend services have their .env files configured (templates are provided in backend/cmd/<service>/.env).
   - Ensure the frontend has its .env file (frontend/.env).

### Running the Application

1. **Start the Backend Services:**

   We provide a PowerShell script to spin up the Gateway and all 5 microservices in separate windows.

   ```PowerShell
   ./scripts/start-all.ps1
   ```

   - This will start:
     - **Gateway:** localhost:8080
     - **Auth Service:** localhost:50051
     - **Course Service:** localhost:50052
     - **Enrollment Service:** localhost:50053
     - **Grade Service:** localhost:50054
     - **Admin Service:** localhost:50055

2. **Start the Frontend:**

   ```Bash
    cd frontend
    npm run dev
   ```

   Access the web application at http://localhost:3000 (or the port shown in your terminal).

3. **Stopping the System**

   To stop all running Go backend processes, use the stop script:

   ```PowerShell
   ./scripts/stop-all.ps1
   ```
