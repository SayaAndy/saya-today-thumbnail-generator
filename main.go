package main

import (
	"flag"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

var (
	configPath = flag.String("c", "config.json", "Path to the configuration file")
)

func main() {
	flag.Parse()

	cfg := &config.Config{}
	if err := config.LoadConfig(*configPath, cfg); err != nil {
		panic(err)
	}
}
