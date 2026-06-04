@echo off
echo =========================================
echo 🚀 Starting Deployment to Google Cloud Run
echo =========================================

echo.
echo [1/2] Deploying User Service...
cd users
call gcloud run deploy user-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [2/2] Deploying API Gateway...
cd api-gateway
call gcloud run deploy api-gateway --source . --region asia-southeast1 --allow-unauthenticated --set-env-vars USER_SERVICE_URL="https://user-service-879165280409.asia-southeast1.run.app"
cd ..

echo.
echo =========================================
echo ✅ All services deployed successfully!
echo =========================================
pause