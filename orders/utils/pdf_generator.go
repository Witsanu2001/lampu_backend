package utils

import (
	"context"
	"fmt"
	"io"
	"orders/models"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/signintech/gopdf"
)

func FixThai(text string) string {
	runes := []rune(text)
	var out []rune
	for i, r := range runes {
		// เช็กว่าเป็นวรรณยุกต์ (่ ้ ๊ ๋ ์) หรือไม่
		if r >= 0x0E48 && r <= 0x0E4C {
			if i > 0 {
				prev := runes[i-1]
				// ถ้าตัวอักษรก่อนหน้าเป็นสระบน (ิ ี ึ ื ั ํ ็)
				if (prev >= 0x0E34 && prev <= 0x0E37) || prev == 0x0E31 || prev == 0x0E4D || prev == 0x0E47 {
					// แปลงรหัสวรรณยุกต์ให้เป็นตัวยกสูงพิเศษ
					out = append(out, 0xF70A+(r-0x0E48))
					continue
				}
			}
		}
		out = append(out, r)
	}
	return string(out)
}

func GenerateOrderPDF(orders []models.Order) (string, error) {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	// 🌟 2. เปลี่ยนชื่อไฟล์ฟอนต์ตรงนี้ให้เป็น THSarabunNew
	err := pdf.AddTTFFont("Sarabun", "fonts/THSarabunNew.ttf")
	if err != nil {
		return "", fmt.Errorf("ไม่สามารถโหลดฟอนต์ได้: %v", err)
	}

	pdf.AddPage()
	yPos := 40.0

	// --- 1. หัวเอกสาร ---
	pdf.SetFont("Sarabun", "", 18)
	pdf.SetXY(40, yPos)
	pdf.Cell(nil, FixThai("รายงานสรุปยอดขายและคำสั่งซื้อสำเร็จ")) // 🌟 ครอบ FixThai
	yPos += 25

	colWidths := []float64{55, 75, 115, 110, 50, 50, 60}
	headers := []string{"วันที่", "ลูกค้า", "รายการหลัก", "เมนูเพิ่มเติม", "ค่าสินค้า", "ค่าส่ง", "ยอดสุทธิ"}

	splitLines := func(text string, width float64) []string {
		var finalLines []string
		rawLines := strings.Split(text, "\n")
		for _, raw := range rawLines {
			lines, _ := pdf.SplitText(raw, width)
			finalLines = append(finalLines, lines...)
		}
		return finalLines
	}

	calculateRowHeight := func(texts []string) float64 {
		maxLines := 1
		for i, txt := range texts {
			lines := splitLines(FixThai(txt), colWidths[i]-8) // 🌟 ครอบ FixThai
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		return float64(maxLines)*14.0 + 6.0
	}

	drawCell := func(x, y, w, h float64, text string, align int) {
		pdf.Line(x, y, x+w, y)
		pdf.Line(x, y+h, x+w, y+h)
		pdf.Line(x, y, x, y+h)
		pdf.Line(x+w, y, x+w, y+h)

		// 🌟 ครอบ FixThai ก่อนวาดลงช่อง
		fixedText := FixThai(text)
		lines := splitLines(fixedText, w-8)
		currentY := y + 5

		for _, line := range lines {
			pdf.SetXY(x+4, currentY)
			if align == 1 {
				textSize, _ := pdf.MeasureTextWidth(line)
				pdf.SetXY(x+w-textSize-4, currentY)
			}
			pdf.Cell(nil, line)
			currentY += 14
		}
	}

	drawHeader := func(startY float64) float64 {
		pdf.SetFont("Sarabun", "", 10)
		pdf.SetLineWidth(0.5)
		currentX := 40.0
		for i, head := range headers {
			drawCell(currentX, startY, colWidths[i], 20, head, 0)
			currentX += colWidths[i]
		}
		return startY + 20
	}

	yPos = drawHeader(yPos)

	var totalOrderSum float64
	var totalShippingSum float64
	var totalGrandSum float64

	pdf.SetFont("Sarabun", "", 9)

	for _, order := range orders {
		orderDate := order.CreatedAt.Format("02/01/06")

		recipient := order.Shipping.Recipient
		if recipient == "" {
			recipient = "-"
		}

		mainItemsStr := ""
		for i, item := range order.MainItems {
			if i > 0 {
				mainItemsStr += "\n"
			}
			mainItemsStr += fmt.Sprintf("%s(x%d)", item.Name, item.Quantity)
		}
		if mainItemsStr == "" {
			mainItemsStr = "-"
		}

		addOnsStr := ""
		for i, item := range order.AddOnItems {
			if i > 0 {
				addOnsStr += "\n"
			}
			addOnsStr += fmt.Sprintf("%s(x%d) ฿%.0f", item.Name, item.Quantity, item.Subtotal)
		}
		if addOnsStr == "" {
			addOnsStr = "-"
		}

		orderItemsTotal := order.Totals.CartTotal + order.Totals.AddOnTotal
		strOrderItemsTotal := fmt.Sprintf("฿%.0f", orderItemsTotal)
		strShipping := fmt.Sprintf("฿%.0f", order.Totals.ShippingFee)
		strGrandTotal := fmt.Sprintf("฿%.0f", order.Totals.GrandTotal)

		rowTexts := []string{orderDate, recipient, mainItemsStr, addOnsStr, strOrderItemsTotal, strShipping, strGrandTotal}
		rowHeight := calculateRowHeight(rowTexts)

		if yPos+rowHeight > 780 {
			pdf.AddPage()
			yPos = 40.0
			yPos = drawHeader(yPos)
			pdf.SetFont("Sarabun", "", 9)
		}

		currentX := 40.0
		drawCell(currentX, yPos, colWidths[0], rowHeight, rowTexts[0], 0)
		currentX += colWidths[0]
		drawCell(currentX, yPos, colWidths[1], rowHeight, rowTexts[1], 0)
		currentX += colWidths[1]
		drawCell(currentX, yPos, colWidths[2], rowHeight, rowTexts[2], 0)
		currentX += colWidths[2]
		drawCell(currentX, yPos, colWidths[3], rowHeight, rowTexts[3], 0)
		currentX += colWidths[3]
		drawCell(currentX, yPos, colWidths[4], rowHeight, rowTexts[4], 1)
		currentX += colWidths[4]
		drawCell(currentX, yPos, colWidths[5], rowHeight, rowTexts[5], 1)
		currentX += colWidths[5]
		drawCell(currentX, yPos, colWidths[6], rowHeight, rowTexts[6], 1)

		totalOrderSum += orderItemsTotal
		totalShippingSum += order.Totals.ShippingFee
		totalGrandSum += order.Totals.GrandTotal

		yPos += rowHeight
	}

	pdf.SetFont("Sarabun", "", 10)

	if yPos+25 > 780 {
		pdf.AddPage()
		yPos = 40.0
	}

	currentX := 40.0
	combinedWidth := colWidths[0] + colWidths[1] + colWidths[2] + colWidths[3]

	pdf.Line(currentX, yPos, currentX+combinedWidth, yPos)
	pdf.Line(currentX, yPos+20, currentX+combinedWidth, yPos+20)
	pdf.Line(currentX, yPos, currentX, yPos+20)
	pdf.Line(currentX+combinedWidth, yPos, currentX+combinedWidth, yPos+20)

	pdf.SetXY(currentX+4, yPos+4)
	pdf.Cell(nil, FixThai("รวมทั้งสิ้น")) // 🌟 ครอบ FixThai
	currentX += combinedWidth

	drawCell(currentX, yPos, colWidths[4], 20, fmt.Sprintf("฿%.0f", totalOrderSum), 1)
	currentX += colWidths[4]
	drawCell(currentX, yPos, colWidths[5], 20, fmt.Sprintf("฿%.0f", totalShippingSum), 1)
	currentX += colWidths[5]
	drawCell(currentX, yPos, colWidths[6], 20, fmt.Sprintf("฿%.0f", totalGrandSum), 1)

	filePath := "temp_orders_report.pdf"
	err = pdf.WritePdf(filePath)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func UploadPDFToStorage(ctx context.Context, localFilePath string, destFileName string) (string, error) {
	bucketName := "lampu-5a178.firebasestorage.app" // ชื่อ Bucket ของคุณ

	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("ไม่สามารถสร้าง storage client ได้: %v", err)
	}
	defer client.Close()

	f, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("ไม่สามารถเปิดไฟล์ local ได้: %v", err)
	}
	defer f.Close()

	bucket := client.Bucket(bucketName)
	obj := bucket.Object(destFileName)
	wc := obj.NewWriter(ctx)
	wc.ContentType = "application/pdf"

	// 4. เริ่มอัปโหลดไฟล์
	if _, err = io.Copy(wc, f); err != nil {
		return "", fmt.Errorf("อัปโหลดไม่สำเร็จ: %v", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("ปิดไฟล์ไม่สำเร็จ: %v", err)
	}

	// 🌟 5. เพิ่มโค้ดตรงนี้: ตั้งค่าให้ไฟล์นี้เป็น Public (อ่านได้อย่างเดียว)
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		// ถ้าแอบตั้งค่าไม่ได้ ให้ print บอกไว้ (แต่ระบบจะไม่พัง)
		fmt.Printf("คำเตือน: ไม่สามารถตั้งค่า Public ACL ได้: %v\n", err)
	}

	// 6. สร้าง URL ดาวน์โหลดตรง
	downloadURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, destFileName)

	return downloadURL, nil
}
