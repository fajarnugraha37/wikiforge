package main

import (
	"github.com/fajarnugraha37/wikiforge/internal/app"
	"os"
)

func main() { os.Exit(app.Run(os.Args[1:])) }
