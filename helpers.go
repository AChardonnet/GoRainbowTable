package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
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
	scrambled := uint64(i) * 0x9E3779B97F4A7C15 //golden ratio

	result := make([]byte, passwordLength)
	charsetLen := uint64(len(charset))

	for j := 0; j < passwordLength; j++ {
		result[j] = charset[scrambled%charsetLen]
		scrambled /= charsetLen
		scrambled ^= 0xBF58476D1CE4E5B9 // Splitmix64
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

func readTableHeader(filename string) (FileHeader, string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return FileHeader{}, "", err
	}
	defer file.Close()

	var header FileHeader
	binary.Read(file, binary.BigEndian, &header)

	charsetBytes := make([]byte, header.CharsetLength)
	io.ReadFull(file, charsetBytes)
	fileCharset := string(charsetBytes)

	return header, fileCharset, nil
}

func printTable(table []TableEntry) {
	for i, entry := range table {
		fmt.Printf("Entry %d | Start : %-10s End : %s\n",
			i,
			string(entry.Start),
			hex.EncodeToString(entry.End[:]),
		)
	}
}

func readFirstTableEntries(filename string, n int) ([]TableEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	header, charset, err := readTableHeader(filename)
	if err != nil {
		return nil, err
	}

	headerSize := uint32(binary.Size(header))
	charsetSize := uint32(len(charset))
	totalBeforePadding := headerSize + charsetSize
	paddingSize := (8 - (totalBeforePadding % 8)) % 8

	dataStartOffset := int64(totalBeforePadding + paddingSize)

	_, err = file.Seek(dataStartOffset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	table := make([]TableEntry, 0, n)
	for i := 0; i < n; i++ {
		entry := TableEntry{
			Start: make([]byte, header.PasswordLength),
		}

		if _, err := io.ReadFull(file, entry.Start); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if _, err := io.ReadFull(file, entry.End[:]); err != nil {
			return nil, err
		}

		table = append(table, entry)
	}
	return table, nil
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

func calculateSuccessProbability(chains int, length int, passwordLength int, charset string) float64 {
	keyspace := math.Pow(float64(len(charset)), float64(passwordLength))

	currentM := float64(chains)
	var totalUniquePoints float64 = 0

	for i := 0; i < length; i++ {
		totalUniquePoints += currentM

		currentM = keyspace * (1 - math.Exp(-currentM/keyspace))
	}

	probability := totalUniquePoints / keyspace

	if probability > 1.0 {
		return 1.0
	}
	return probability
}

func calculateRequiredChains(targetProb float64, chainLen int, passLen int, charset string) uint64 {
	if targetProb >= 1.0 {
		targetProb = 0.9999 // Probability cannot be 1.0 (infinite chains)
	}

	// Keyspace Size (M = charsetLength ^ passwordLength)
	keyspace := math.Pow(float64(len(charset)), float64(passLen))

	// N = -(M * ln(1 - P)) / L
	n := -(keyspace * math.Log(1-targetProb)) / float64(chainLen)

	return uint64(math.Ceil(n))
}
