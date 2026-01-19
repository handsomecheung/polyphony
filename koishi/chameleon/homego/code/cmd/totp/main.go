package main

import (
	"fmt"
	"log"

	"koishi/chameleon/homego/internal/auth"
)

func main() {
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		log.Fatalf("Failed to generate TOTP secret: %v", err)
	}
	fmt.Println(secret)
}
