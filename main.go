package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const (
	charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
	passwordLength = 6
	chainLength    = 1000
)

func main() {
	generateChain("a")
}

type TableEntry struct {
	Start string
	End   [32]byte
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

func hash(password string) [32]byte {
	hash := sha256.Sum256([]byte(password))
	return hash
}

func reduce(hash [32]byte, column int) string {
	val := binary.BigEndian.Uint64(hash[:8])
	val += uint64(column)

	result := make([]byte, passwordLength)
	for i := 0; i < passwordLength; i++ {
		result[i] = charset[val%uint64(len(charset))]
		val /= uint64(len(charset))
	}
	return string(result)
}

func generateChain(startPlain string) TableEntry {
	currentPlain := startPlain
	var currentHash [32]byte

	for i := 0; i < chainLength; i++ {
		currentHash = hash(currentPlain)

		fmt.Printf("Round %d | Plain : %s Hash : %s\n", i, currentPlain, hex.EncodeToString(currentHash[:]))

		if i < chainLength-1 {
			currentPlain = reduce(currentHash, i)
		}
	}

	return TableEntry{
		Start: startPlain,
		End:   currentHash,
	}
}
