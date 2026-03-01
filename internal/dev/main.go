package main

import (
	"github.com/ghobs91/lodestone/internal/dev/app"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	app.New().Run()
}
