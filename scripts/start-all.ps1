Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Starting College Enrollment System Services" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# Get the project root directory
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$BackendPath = Join-Path $ProjectRoot "backend"

# Array to store all running processes
$Global:ServiceProcesses = @()

function Start-Service {
    param(
        [string]$ServiceName,
        [string]$ServicePath,
        [string]$Port
    )
    
    Write-Host "Starting $ServiceName on port $Port..." -ForegroundColor Yellow
    
    # Check if port is already in use
    $portInUse = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
    if ($portInUse) {
        Write-Host "  Warning: Port $Port is already in use!" -ForegroundColor Red
        return $null
    }
    
    # Start the service in a new PowerShell window
    $process = Start-Process powershell -ArgumentList @(
        "-NoExit",
        "-Command",
        "cd '$ServicePath'; Write-Host 'Starting $ServiceName...' -ForegroundColor Green; go run ."
    ) -PassThru -WindowStyle Normal
    
    if ($process) {
        Write-Host "  $ServiceName started (PID: $($process.Id))" -ForegroundColor Green
        Start-Sleep -Seconds 2
        return $process
    }
    else {
        Write-Host "  Failed to start $ServiceName" -ForegroundColor Red
        return $null
    }
}

# Start Auth Service
$authPath = Join-Path $BackendPath "auth-service"
$authProc = Start-Service -ServiceName "Auth Service" -ServicePath $authPath -Port "50051"
if ($authProc) { $Global:ServiceProcesses += $authProc }

# Start Course Service
$coursePath = Join-Path $BackendPath "course-service"
$courseProc = Start-Service -ServiceName "Course Service" -ServicePath $coursePath -Port "50052"
if ($courseProc) { $Global:ServiceProcesses += $courseProc }

# Start Enrollment Service
$enrollPath = Join-Path $BackendPath "enrollment-service"
$enrollProc = Start-Service -ServiceName "Enrollment Service" -ServicePath $enrollPath -Port "50053"
if ($enrollProc) { $Global:ServiceProcesses += $enrollProc }

# Start Grade Service
$gradePath = Join-Path $BackendPath "grade-service"
$gradeProc = Start-Service -ServiceName "Grade Service" -ServicePath $gradePath -Port "50054"
if ($gradeProc) { $Global:ServiceProcesses += $gradeProc }

# Start Admin Service
$adminPath = Join-Path $BackendPath "admin-service"
$adminProc = Start-Service -ServiceName "Admin Service" -ServicePath $adminPath -Port "50055"
if ($adminProc) { $Global:ServiceProcesses += $adminProc }

# Wait a bit for services to start
Write-Host ""
Write-Host "Waiting for services to initialize..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Start Gateway
$gatewayPath = Join-Path $BackendPath "gateway"
$gatewayProc = Start-Service -ServiceName "Gateway" -ServicePath $gatewayPath -Port "8080"
if ($gatewayProc) { $Global:ServiceProcesses += $gatewayProc }

Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "All services started successfully!" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Services running:" -ForegroundColor Cyan
Write-Host "  - Auth Service:       localhost:50051" -ForegroundColor White
Write-Host "  - Course Service:     localhost:50052" -ForegroundColor White
Write-Host "  - Enrollment Service: localhost:50053" -ForegroundColor White
Write-Host "  - Grade Service:      localhost:50054" -ForegroundColor White
Write-Host "  - Admin Service:      localhost:50055" -ForegroundColor White
Write-Host "  - Gateway (HTTP):     localhost:8080" -ForegroundColor White
Write-Host ""
Write-Host "Press Ctrl+C in each window to stop services" -ForegroundColor Yellow
Write-Host "Or run './scripts/stop-all.ps1' to stop all services" -ForegroundColor Yellow