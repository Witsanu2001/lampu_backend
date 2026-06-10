@echo off
echo =========================================
echo 🚀 Starting Deployment to Google Cloud Run
echo =========================================

echo.
echo [1/3] Deploying User Service...
cd users
call gcloud run deploy user-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [2/3] Deploying Order Service...
cd orders
call gcloud run deploy order-service --source . --region asia-southeast1 --allow-unauthenticated
cd ..

echo.
echo [3/3] Deploying API Gateway...
cd api-gateway
:: 🎯 แก้ไขบรรทัดนี้: เพิ่ม ORDERS_SERVICE_URL เข้าไปคั่นด้วยลูกน้ำ
call gcloud run deploy api-gateway --source . --region asia-southeast1 --allow-unauthenticated --set-env-vars="USER_SERVICE_URL=https://user-service-879165280409.asia-southeast1.run.app,ORDERS_SERVICE_URL=https://order-service-879165280409.asia-southeast1.run.app"
cd ..

echo.
echo =========================================
echo ✅ All services deployed successfully!
echo =========================================
pause