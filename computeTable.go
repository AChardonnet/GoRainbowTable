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

	"github.com/schollz/progressbar/v3"
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

	tablesDir := filepath.Join(dir, "tables")
	err = os.MkdirAll(tablesDir, 0755)
	if err != nil {
		return err
	}

	creationTime := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(tablesDir, creationTime+".rtable")

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

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	tablesDir := filepath.Join(dir, "tables")
	err = os.MkdirAll(tablesDir, 0755)
	if err != nil {
		log.Fatal(err)
	}

	creationTime := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(tablesDir, creationTime+".rtable")

	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	endHashes := make(map[[32]byte]struct{}, chainsNumber)
	collisions := 0

	for i := 0; i < chainsNumber; i++ {
		chain := generateChain(seed(i))
		printIfVerbose(isVerbose, "Chain %d | Start : %s End : %s\n", i, chain.Start, hex.EncodeToString(chain.End[:]))

		if _, exists := endHashes[chain.End]; exists {
			collisions++
		} else {
			endHashes[chain.End] = struct{}{}
			err := binary.Write(file, binary.BigEndian, chain)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	printIfVerbose(isVerbose, "Generation complete. Collisions: %d\n", collisions)
	printIfVerbose(isVerbose, "Table saved to %s\n", path)
}

func worker(id int, jobs <-chan string, results chan<- TableEntry, wg *sync.WaitGroup) {
	defer wg.Done()
	for start := range jobs {
		results <- generateChain(start)
	}
}

func generateTableMultiThread(verbose ...bool) {
	isVerbose := false
	if len(verbose) > 0 {
		isVerbose = true
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	tablesDir := filepath.Join(dir, "tables")
	err = os.MkdirAll(tablesDir, 0755)
	if err != nil {
		log.Fatal(err)
	}

	creationTime := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(tablesDir, creationTime+".rtable")

	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	jobs := make(chan string, 100)
	results := make(chan TableEntry, 100)
	var wg sync.WaitGroup

	printIfVerbose(isVerbose, "Creating Workers... ")

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	printIfVerbose(isVerbose, "Done \n")
	bar := progressbar.Default(int64(chainsNumber), "Generating Table")
	printIfVerbose(isVerbose, "Starting Jobs")

	done := make(chan bool)
	go collectResults(results, done, isVerbose, bar, file)

	for i := 0; i < chainsNumber; i++ {
		jobs <- seed(i)
	}
	close(jobs)

	wg.Wait()
	close(results)
	<-done

	printIfVerbose(isVerbose, "Chains Generated\n")
	printIfVerbose(isVerbose, "Table saved to %s\n", path)
}

func collectResults(results <-chan TableEntry, done chan bool, isVerbose bool, bar *progressbar.ProgressBar, file *os.File) {
	endHashes := make(map[[32]byte]struct{}, chainsNumber)
	collisions := 0
	updateBar := 0

	for entry := range results {
		if _, exists := endHashes[entry.End]; exists {
			collisions++
		} else {
			endHashes[entry.End] = struct{}{}
			err := binary.Write(file, binary.BigEndian, entry)
			if err != nil {
				log.Fatal(err)
			}
		}
		if updateBar%1000 == 0 {
			bar.Add(1000)
		}
		updateBar++
	}
	printIfVerbose(isVerbose, "Generation complete. Collisions: %d\n", collisions)
	done <- true
}
