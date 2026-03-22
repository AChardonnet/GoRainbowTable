package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
)

type FileHeader struct {
	Magic          [4]byte
	Version        uint32
	PasswordLength uint32
	ChainLength    uint32
	NumChains      uint64
	CharsetLength  uint32
	IsSorted       uint8
	Padding        [3]byte
}

type TableEntry struct {
	Start []byte
	End   [32]byte
}

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

func reduce(hash [32]byte, column int, passwordLength int, charset string) string {
	val := binary.BigEndian.Uint64(hash[:8])
	val += uint64(column)

	result := make([]byte, passwordLength)
	for i := 0; i < passwordLength; i++ {
		result[i] = charset[val%uint64(len(charset))]
		val /= uint64(len(charset))
	}
	return string(result)
}

func seed(i int, passwordLength int, charset string) string {
	index := uint64(i)
	result := make([]byte, passwordLength)
	for i := 0; i < passwordLength; i++ {
		result[i] = charset[index%uint64(len(charset))]
		index /= uint64(len(charset))
	}
	return string(result)
}

func loadTableWithHeader(filename string) (FileHeader, string, []TableEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return FileHeader{}, "", nil, err
	}
	defer file.Close()

	var header FileHeader
	binary.Read(file, binary.BigEndian, &header)

	charsetBytes := make([]byte, header.CharsetLength)
	io.ReadFull(file, charsetBytes)
	fileCharset := string(charsetBytes)

	headerSize := uint32(binary.Size(FileHeader{}))
	currentPos := headerSize + header.CharsetLength
	paddingSize := (8 - (currentPos % 8)) % 8

	if paddingSize > 0 {
		file.Seek(int64(paddingSize), io.SeekCurrent)
	}

	table := make([]TableEntry, 0, header.NumChains)
	for i := uint64(0); i < header.NumChains; i++ {
		startBuf := make([]byte, header.PasswordLength)
		if _, err := io.ReadFull(file, startBuf); err != nil {
			break
		}
		var endHash [32]byte
		if _, err := io.ReadFull(file, endHash[:]); err != nil {
			break
		}

		entry := TableEntry{
			Start: startBuf,
			End:   endHash,
		}

		table = append(table, entry)
	}

	return header, fileCharset, table, nil
}

func printTable(table []TableEntry) {
	for _, entry := range table {
		fmt.Printf("Start : %s End : %s \n", entry.Start, hex.EncodeToString(entry.End[:]))
	}
}

func saveTableWithHeader(
	path string, table []TableEntry, isSorted bool, usedCharset string, passwordLength int, chainLength int,
) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	headerSize := uint32(binary.Size(FileHeader{}))
	charsetSize := uint32(len(usedCharset))
	totalBeforePadding := headerSize + charsetSize

	paddingSize := (8 - (totalBeforePadding % 8)) % 8

	header := FileHeader{
		Magic:          [4]byte{'R', 'B', 'O', 'W'},
		Version:        1,
		PasswordLength: uint32(passwordLength),
		ChainLength:    uint32(chainLength),
		NumChains:      uint64(len(table)),
		CharsetLength:  uint32(len(usedCharset)),
		IsSorted:       0,
	}
	if isSorted {
		header.IsSorted = 1
	}

	binary.Write(file, binary.BigEndian, header)

	file.Write([]byte(usedCharset))

	if paddingSize > 0 {
		padding := make([]byte, paddingSize)
		file.Write(padding)
	}

	for _, entry := range table {
		file.Write(entry.Start)
		file.Write(entry.End[:])
	}
	return nil
}

func sortTable(table []TableEntry) []TableEntry {
	sort.Slice(table, func(i, j int) bool {
		return bytes.Compare(table[i].End[:], table[j].End[:]) < 0
	})
	return table
}
