package main

import (
	"github.com/example/wikiforge/internal/app"
	"os"
)

func main() { os.Exit(app.Run(os.Args[1:])) }
