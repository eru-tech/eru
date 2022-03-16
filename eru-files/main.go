package main

import (
	"github.com/eru-tech/eru/eru-files/server"
	"log"
)

func main() {
	e := server.Init()
	if e != nil {
		log.Print(e)
	}
}
