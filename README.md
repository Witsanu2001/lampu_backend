# กรณี deploy ตัวจริง ให้ push ขึ้น github แล้วระบบ github actions จะทำการ deploy ให้อัตโนมัติ

#เปลี่ยน branch เป็น dev หรือ แตก branch ใหม่

#1. กลับไปตั้งหลักที่ main ก่อน (เผื่อคุณเผลอไปอยู่สาขาอื่น)
git checkout main

#2. ดึงโค้ดล่าสุดจาก GitHub ลงมาอัปเดตเครื่องเรา (ป้องกันโค้ดชนกัน)
git pull origin main

#3. แตก Branch ใหม่ชื่อ dev และย้ายตัวเองไปอยู่สาขานั้นทันที!
git checkout -b dev

#4. เอาสาขา dev ดันขึ้นไปเก็บไว้บน GitHub ด้วย
git push -u origin dev


#เปลี่ยนไปเป็น main 
git checkout main 
#หรือ ถ้าเป็น Git เวอร์ชันใหม่ๆ จะใช้คำสั่ง 
git switch main #ก็ได้ครับ ผลลัพธ์เหมือนกันเป๊ะ

#ถ้ามีงานที่เขียนค้างไว้ใน dev: ก่อนจะสลับกลับไป main แนะนำให้เคลียร์งานใน dev ให้เรียบร้อยก่อนครับ โดยการพิมพ์เซฟงานไว้:

git add .
git commit -m "add service order"
git push origin

#อยากสลับกลับไปทำงานที่ dev อีกรอบทำยังไง

git checkout dev