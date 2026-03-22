package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

func printIfVerbose(isVerbose bool, format string, a ...any) {
	if isVerbose {
		fmt.Printf(format, a...)
	}
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
	return sha256.Sum256([]byte(password))
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

func seed(i int) string {
	index := uint64(i)
	result := make([]byte, passwordLength)
	for i := 0; i < passwordLength; i++ {
		result[i] = charset[index%uint64(len(charset))]
		index /= uint64(len(charset))
	}
	return string(result)
}
