package main

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type row struct {
	ID           string
	Pool         string
	Status       string
	AccountEmail string
	ImageLimited bool
	VideoLimited bool
}

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		fmt.Println("POSTGRES_DSN env is required, e.g. host=127.0.0.1 user=postgres password=... dbname=vivid_ai port=5432 sslmode=disable TimeZone=Asia/Shanghai")
		os.Exit(1)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("open err:", err)
		os.Exit(1)
	}

	var pick row
	db.Raw(`SELECT id, pool, status, account_email, image_limited, video_limited FROM token_accounts WHERE pool='adobe' ORDER BY id LIMIT 1`).Scan(&pick)
	fmt.Printf("picked: id=%s email=%s status=%s image_limited=%v video_limited=%v\n", pick.ID, pick.AccountEmail, pick.Status, pick.ImageLimited, pick.VideoLimited)

	if err := db.Exec(`UPDATE token_accounts SET video_limited=true, updated_at=now() WHERE id=?`, pick.ID).Error; err != nil {
		fmt.Println("update err:", err)
		os.Exit(1)
	}
	fmt.Println("-> set video_limited=true")

	var after row
	db.Raw(`SELECT id, pool, status, account_email, image_limited, video_limited FROM token_accounts WHERE id=?`, pick.ID).Scan(&after)
	fmt.Printf("after:  id=%s status=%s image_limited=%v video_limited=%v\n", after.ID, after.Status, after.ImageLimited, after.VideoLimited)
}
