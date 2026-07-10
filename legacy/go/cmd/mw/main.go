package main

import (
	"log"
	"os"

	"github.com/Noswad123/mind-weaver/internal/mwcli"
)

func main() {
	if err := mwcli.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
