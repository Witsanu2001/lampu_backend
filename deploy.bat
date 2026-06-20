@echo off
echo =========================================
echo 🚀 Starting Deployment to Google Cloud Run
echo =========================================

echo.
echo [1/5] Deploying User Service...
cd users
call gcloud run deploy user-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [2/5] Deploying Order Service...
cd orders
call gcloud run deploy order-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [3/5] Deploying Job Service...
cd jobs
call gcloud run deploy job-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [4/5] Deploying Job Service...
cd jobs
call gcloud run deploy job-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [5/5] Deploying API Gateway...
cd api-gateway
call gcloud run deploy api-gateway --source . --region asia-southeast1 --allow-unauthenticated --set-env-vars="USER_SERVICE_URL=https://user-service-879165280409.asia-southeast1.run.app,ORDERS_SERVICE_URL=https://order-service-879165280409.asia-southeast1.run.app,JOBS_SERVICE_URL=https://job-service-879165280409.asia-southeast1.run.app,SYSTEMS_SERVICE_URL=https://system-service-879165280409.asia-southeast1.run.app"
cd ..

echo.
echo =========================================
echo ✅ All services deployed successfully!
echo =========================================
pause