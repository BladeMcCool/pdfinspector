@echo off
setlocal

:: Debugging on to see the commands as they are executed
@echo on

:: Capture start time
echo Starting build process at %TIME%.
gcloud builds submit --tag gcr.io/astute-backup-434623-h3/pdfinspector
echo Build successful!
echo Completed build process at %TIME%.
