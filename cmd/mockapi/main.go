package main

import (
	"log"
	"os"

	"testforge/mockapi"
)

func main() {
	addr := os.Getenv("MOCKAPI_ADDR")
	if addr == "" {
		addr = ":8099"
	}
	log.Printf("mockapi listening on %s", addr)
	log.Fatal(mockapi.Start(addr))
}
