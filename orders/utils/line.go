package utils

import (
	"fmt"
	"log"

	"github.com/line/line-bot-sdk-go/v7/linebot"
)

// สังเกตว่าตัว S ตัวใหญ่ เพื่อให้แพ็กเกจอื่นเรียกใช้ได้
func SendOrderNotification(riderUserID string, orderDetails string) error {
	// ใส่ Token ของคุณ
	bot, err := linebot.New("125bcfec8cc54507f8b01df805b58bbf", "EH23/1aroL1pZ1gtRKc0eFu7fiYieMlGkKzrSldkHPvdn21tlbfVUQ9KmurIbInY6lKQkQ7MfzYDtC4fe1jk2emvyPICyWBr8k2bbn8ALkv50ctx+/yxVTZhkDK4IqRG+rodTN8eqhWnpM1Nopr9MwdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		return fmt.Errorf("failed to create line bot: %v", err)
	}

	message := linebot.NewTextMessage(orderDetails)

	_, err = bot.PushMessage(riderUserID, message).Do()
	if err != nil {
		return fmt.Errorf("failed to push message: %v", err)
	}

	log.Println("ส่งแจ้งเตือนเข้า LINE เรียบร้อย!")
	return nil
}

func SendOrderAdminNotification(adminUID string, orderDetails string) error {
	// ใส่ Token ของคุณ
	bot, err := linebot.New("b265bf9a0b64d6ff8844f1f4c8c9ce0d", "utFIjp6YeTghCpmCi5+fH65Ib8iFusrnIV3PTbhoQhMyvyU/gkYIbn6uNgg8npyN72QLI12ogPw2vFL/w6cp5Fnpzi43q6JgjIz2HW2dIM2PywIbt2ZsXLlyYKUOQBOwcW3A03L84j8WJRtcWcnIxgdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		return fmt.Errorf("failed to create line bot: %v", err)
	}

	message := linebot.NewTextMessage(orderDetails)

	_, err = bot.PushMessage(adminUID, message).Do()
	if err != nil {
		return fmt.Errorf("failed to push message: %v", err)
	}

	log.Println("ส่งแจ้งเตือนเข้า LINE ร้านเรียบร้อย!")
	return nil
}

func SendOrderUserNotification(adminUID string, orderDetails string) error {
	// ใส่ Token ของคุณ
	bot, err := linebot.New("5b66c6bb65c99a9603ada6f3ef62661f", "jzTtllClFeqhwNIQkKRV9vagwTeTaGgJHIIqT6SWDShvb2amzZznj1/yxe5oIwVQeWpo1lsQds+9ZNPg9xxzCwbMf+Yuc5T0IG6/ivVF46xEgEhEzNZ8Tju+CUQyyjomIbaX6Kro9wARNLJUeF6WYgdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		return fmt.Errorf("failed to create line bot: %v", err)
	}

	message := linebot.NewTextMessage(orderDetails)

	_, err = bot.PushMessage(adminUID, message).Do()
	if err != nil {
		return fmt.Errorf("failed to push message: %v", err)
	}

	log.Println("ส่งแจ้งเตือนเข้า LINE ร้านเรียบร้อย!")
	return nil
}

func SendOrderRiderNotification(adminUID string, orderDetails string) error {
	// ใส่ Token ของคุณ
	bot, err := linebot.New("125bcfec8cc54507f8b01df805b58bbf", "EH23/1aroL1pZ1gtRKc0eFu7fiYieMlGkKzrSldkHPvdn21tlbfVUQ9KmurIbInY6lKQkQ7MfzYDtC4fe1jk2emvyPICyWBr8k2bbn8ALkv50ctx+/yxVTZhkDK4IqRG+rodTN8eqhWnpM1Nopr9MwdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		return fmt.Errorf("failed to create line bot: %v", err)
	}

	message := linebot.NewTextMessage(orderDetails)

	_, err = bot.PushMessage(adminUID, message).Do()
	if err != nil {
		return fmt.Errorf("failed to push message: %v", err)
	}

	log.Println("ส่งแจ้งเตือนเข้า LINE ร้านเรียบร้อย!")
	return nil
}
