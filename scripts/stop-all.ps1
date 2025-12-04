Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Stopping College Enrollment System Services" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# Get all Go processes
$goProcesses = Get-Process -Name "go" -ErrorAction SilentlyContinue

if ($goProcesses) {
    Write-Host "Found $($goProcesses.Count) Go process(es) running" -ForegroundColor Yellow
    
    foreach ($process in $goProcesses) {
        try {
            Write-Host "Stopping process $($process.Id)..." -ForegroundColor Yellow
            Stop-Process -Id $process.Id -Force
            Write-Host "  Process $($process.Id) stopped" -ForegroundColor Green
        }
        catch {
            Write-Host "  Failed to stop process $($process.Id): $_" -ForegroundColor Red
        }
    }
}
else {
    Write-Host "No Go processes found" -ForegroundColor Yellow
}

# Also kill any PowerShell windows that might be running services
# Updated to match the explicit Window Titles set in start-all.ps1
$serviceWindows = Get-Process -Name "powershell" -ErrorAction SilentlyContinue | 
Where-Object { $_.MainWindowTitle -match "Course Service|Auth Service|Enrollment Service|Grade Service|Admin Service|Gateway" }

if ($serviceWindows) {
    Write-Host ""
    Write-Host "Found $($serviceWindows.Count) service window(s)" -ForegroundColor Yellow
    
    foreach ($window in $serviceWindows) {
        try {
            Write-Host "Closing window: $($window.MainWindowTitle)..." -ForegroundColor Yellow
            Stop-Process -Id $window.Id -Force
            Write-Host "  Window closed" -ForegroundColor Green
        }
        catch {
            Write-Host "  Failed to close window: $_" -ForegroundColor Red
        }
    }
}

Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "All services stopped" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green