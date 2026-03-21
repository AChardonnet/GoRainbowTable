package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
	passwordLength = 6
	chainLength    = 1000
	chainsNumber   = 10000
)

func main() {
	generateTable(true)
}

type TableEntry struct {
	Start [passwordLength]byte
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

func generateChain(startPlain string, verbose ...bool) TableEntry {
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	currentPlain := startPlain
	var currentHash [32]byte

	for i := 0; i < chainLength; i++ {
		currentHash = hash(currentPlain)

		if isVerbose {
			fmt.Printf("Round %d | Plain : %s Hash : %s\n", i, currentPlain, hex.EncodeToString(currentHash[:]))
		}

		if i < chainLength-1 {
			currentPlain = reduce(currentHash, i)
		}
	}

	var startBytes [passwordLength]byte
	copy(startBytes[:], startPlain)
	return TableEntry{
		Start: startBytes,
		End:   currentHash,
	}
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

func saveTable(filename string, table []TableEntry) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range table {
		err := binary.Write(file, binary.BigEndian, entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateTable(verbose ...bool) {
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	table := make([]TableEntry, 0, chainsNumber)
	for i := 0; i < chainsNumber; i++ {
		chain := generateChain(seed(i))
		if isVerbose {
			fmt.Printf("Chain %d | Start : %s End : %s\n", i, chain.Start, hex.EncodeToString(chain.End[:]))
		}
		table = append(table, chain)
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Error retrieving current directory:", err)
		return
	}

	creationTime := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, creationTime+".rtable")

	err = saveTable(path, table)
	if err != nil {
		log.Fatal(err)
	}
	if isVerbose {
		fmt.Printf("Table saved to %s\n", path)
	}
}
