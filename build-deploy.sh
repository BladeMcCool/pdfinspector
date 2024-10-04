set -e
docker build -t gcr.io/astute-backup-434623-h3/pdfinspector:latest .
docker push gcr.io/astute-backup-434623-h3/pdfinspector:latest
gcloud run deploy pdfinspector --image gcr.io/astute-backup-434623-h3/pdfinspector \
 --platform managed --region us-central1 --allow-unauthenticated \
 --update-secrets="OPENAI_API_KEY=openai-apikey:latest,ADMIN_KEY=admin-key:latest,FRONTEND_SSO_CLIENT_SECRET=frontend-sso-client-secret:latest,JWT_SECRET=jwt-secret:latest,STRIPE_API_SECRET_KEY=stripe-api-secret-key:latest,STRIPE_WEBHOOK_SECRET=stripe-webhook-secret:latest" \
 --update-env-vars="GOTENBERG_URL=https://gotenberg-1025621488749.us-central1.run.app,JSON_SERVER_URL=https://json-server-1025621488749.us-central1.run.app,REACT_APP_URL=https://react-app-1025621488749.us-central1.run.app,FSTYPE=gcs,USE_SYSTEM_GS=true,FRONTEND_SSO_CLIENT_ID=1025621488749-bsh6v12kgatbcpmoi0hhc5ulpdc4liih.apps.googleusercontent.com"
