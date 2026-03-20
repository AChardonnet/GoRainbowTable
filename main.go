package main

import (
	"crypto/sha256"
	"fmt"
)

func main() {
	const charset = "a-zA-Z0-9!#@+"
	fmt.Println(hash("a"))
}

func generateCharset(regex string) string {
	result := []rune("")
	runes := []rune(regex)

	for i := 0; i < len(runes); i++ {
		if i+2 < len(runes) && runes[i+1] == []rune("-")[0] {
			start := runes[i]
			end := runes[i+2]
			for r := start; r <= end; r++ {
				result = append(result, r)
			}
			i += 2
		} else {
			result = append(result, runes[i])
		}
	}

	return string(result)
}

func hash(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}
