@echo off
setlocal

:: Debugging on to see the commands as they are executed
@echo on

:: Capture start time
echo Starting deployment process at %TIME%.
gcloud run deploy pdfinspector --image gcr.io/astute-backup-434623-h3/pdfinspector --platform managed --region us-central1 --allow-unauthenticated --update-secrets OPENAI_API_KEY=openai-apikey:latest --update-env-vars="GOTENBERG_URL=https://gotenberg-1025621488749.us-central1.run.app,JSON_SERVER_URL=https://json-server-1025621488749.us-central1.run.app,REACT_APP_URL=https://react-app-1025621488749.us-central1.run.app"
echo Deployment successful!
echo Completed deployment process at %TIME%.
