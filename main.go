package main

import (
	"github.com/ghobs91/lodestone/internal/app"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	app.New().Run()
}
