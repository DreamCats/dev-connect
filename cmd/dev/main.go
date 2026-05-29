package main

import (
	"os"

	"github.com/DreamCats/dev-connect/internal/cli"
)

func main() {
	cli.Main(os.Args[1:])
}
