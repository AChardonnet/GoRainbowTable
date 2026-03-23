package main

import (
	"encoding/hex"
	"fmt"
)

func main() {
	myHash := hash("vous")
	fmt.Println(hex.EncodeToString(myHash[:]))
	fmt.Println(displayCharset("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"))
	tui()
}
