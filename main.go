package main

import (
	"fmt"
	"horriya/crypto"
)

func main() {
	fmt.Println("Hi there !")
	identity := crypto.GenerateKey()
	fmt.Printf("Hey new ident ! %x", identity)
}
