package main

import (
	"log"

	"dock-it/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("dock-it: %v", err)
	}
}
