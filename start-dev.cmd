@echo off
start "IMS backend" cmd /k "cd /d %~dp0backend && go run ./cmd/api"
start "IMS frontend" cmd /k "cd /d %~dp0frontend && npm run dev"
