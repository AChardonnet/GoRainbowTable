package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TableEntry struct {
	Start [passwordLength]byte
	End   [32]byte
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

		printIfVerbose(isVerbose, "Round %d | Plain : %s Hash : %s\n", i, currentPlain, hex.EncodeToString(currentHash[:]))

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

func saveTable(table []TableEntry, isVerbose bool) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	creationTime := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, creationTime+".rtable")

	file, err := os.Create(path)
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

	printIfVerbose(isVerbose, "Table saved to %s\n", path)
	return nil
}

func loadTable(filename string) ([]TableEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var table []TableEntry
	for {
		var entry TableEntry
		err := binary.Read(file, binary.BigEndian, &entry)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		table = append(table, entry)
	}
	return table, nil
}

func printTable(table []TableEntry) {
	for _, entry := range table {
		fmt.Printf("Start : %s End : %s \n", entry.Start, hex.EncodeToString(entry.End[:]))
	}
}

func generateTableSingleThread(verbose ...bool) {
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = verbose[0]
	}
	table := make([]TableEntry, 0, chainsNumber)
	for i := 0; i < chainsNumber; i++ {
		chain := generateChain(seed(i))
		printIfVerbose(isVerbose, "Chain %d | Start : %s End : %s\n", i, chain.Start, hex.EncodeToString(chain.End[:]))
		table = append(table, chain)
	}

	err := saveTable(table, isVerbose)
	if err != nil {
		log.Fatal(err)
	}
}

func worker(id int, jobs <-chan string, results chan<- TableEntry, wg *sync.WaitGroup, isVerbose bool) {
	defer wg.Done()
	for start := range jobs {
		results <- generateChain(start)
		printIfVerbose(isVerbose, "Worker : %d | Job finished\n", id)
	}
}

func generateTableMultiThread(verbose ...bool) {
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = true
	}
	jobs := make(chan string, 100)
	results := make(chan TableEntry, 100)
	var wg sync.WaitGroup

	printIfVerbose(isVerbose, "Creating Workers... ")

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg, isVerbose)
	}

	printIfVerbose(isVerbose, "Done \n")
	printIfVerbose(isVerbose, "Starting Jobs")

	table := make([]TableEntry, 0, chainsNumber)
	done := make(chan []TableEntry)
	go collectResults(results, done, isVerbose)

	for i := 0; i < chainsNumber; i++ {
		jobs <- seed(i)
	}
	close(jobs)

	wg.Wait()
	close(results)
	table = <-done

	printIfVerbose(isVerbose, "Chains Generated\n")

	saveTable(table, isVerbose)
}

func collectResults(results <-chan TableEntry, done chan []TableEntry, isVerbose bool) {
	endHashes := make(map[[32]byte]struct{}, chainsNumber)
	finalTable := make([]TableEntry, 0, chainsNumber)

	collisions := 0

	for entry := range results {
		if _, exists := endHashes[entry.End]; exists {
			collisions++
		} else {
			endHashes[entry.End] = struct{}{}
			finalTable = append(finalTable, entry)
		}

	}
	printIfVerbose(isVerbose, "Generation complete. Collisions: %d\n", collisions)
	done <- finalTable
}
