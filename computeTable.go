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

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

func generateChain(startPlain string, chainLength int, passwordLength int, charset string, verbose ...bool) TableEntry {
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
			currentPlain = reduce(currentHash, i, passwordLength, charset)
		}
	}

	return TableEntry{
		Start: []byte(startPlain),
		End:   currentHash,
	}
}

func generateTableSingleThread(chainsNumber int, passwordLength int, charset string, chainLength int, usedCharset string, verbose ...bool) {
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

	headerSize := uint32(binary.Size(FileHeader{}))
	charsetSize := uint32(len(usedCharset))
	totalBeforePadding := headerSize + charsetSize

	paddingSize := (8 - (totalBeforePadding % 8)) % 8

	header := FileHeader{
		Magic:          [4]byte{'R', 'B', 'O', 'W'},
		Version:        1,
		PasswordLength: uint32(passwordLength),
		ChainLength:    uint32(chainLength),
		NumChains:      uint64(0),
		CharsetLength:  uint32(len(usedCharset)),
		IsSorted:       0,
	}

	binary.Write(file, binary.BigEndian, header)

	file.Write([]byte(usedCharset))

	if paddingSize > 0 {
		padding := make([]byte, paddingSize)
		file.Write(padding)
	}

	endHashes := make(map[[32]byte]struct{}, chainsNumber)
	savedChains := uint64(0)

	for i := 0; i < chainsNumber; i++ {
		chain := generateChain(seed(i, passwordLength, charset), chainLength, passwordLength, charset)
		printIfVerbose(isVerbose, "Chain %d | Start : %s End : %s\n", i, chain.Start, hex.EncodeToString(chain.End[:]))

		if _, exists := endHashes[chain.End]; !exists {
			endHashes[chain.End] = struct{}{}

			file.Write(chain.Start)
			file.Write(chain.End[:])

			savedChains++
		}
	}

	header.NumChains = savedChains
	file.Seek(0, io.SeekStart)
	binary.Write(file, binary.BigEndian, header)

	printIfVerbose(isVerbose, "Generation complete. Collisions: %d\n", chainsNumber-int(savedChains))
	printIfVerbose(isVerbose, "Table saved to %s\n", path)
}

func worker(id int, jobs <-chan string, results chan<- TableEntry, wg *sync.WaitGroup, chainLength int, passwordLength int, charset string) {
	defer wg.Done()
	for start := range jobs {
		results <- generateChain(start, chainLength, passwordLength, charset)
	}
}

func generateTableMultiThread(workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int, verbose ...bool) {
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

	header := FileHeader{
		Magic:          [4]byte{'R', 'B', 'O', 'W'},
		Version:        1,
		PasswordLength: uint32(passwordLength),
		ChainLength:    uint32(chainLength),
		NumChains:      0,
		CharsetLength:  uint32(len(charset)),
		IsSorted:       0,
	}
	binary.Write(file, binary.BigEndian, header)
	file.Write([]byte(charset))

	paddingSize := (8 - ((uint32(binary.Size(header)) + uint32(len(charset))) % 8)) % 8
	if paddingSize > 0 {
		file.Write(make([]byte, paddingSize))
	}

	jobs := make(chan string, 100)
	results := make(chan TableEntry, 100)
	countChan := make(chan uint64)
	var wg sync.WaitGroup

	printIfVerbose(isVerbose, "Creating Workers... ")

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg, chainLength, passwordLength, charset)
	}

	printIfVerbose(isVerbose, "Done \n")

	progressBar := mpb.New(mpb.WithAutoRefresh())

	elapsedDec := decor.Elapsed(decor.ET_STYLE_GO)
	etaDec := decor.AverageETA(decor.ET_STYLE_GO)
	bar := progressBar.AddBar(
		int64(chainsNumber),
		mpb.PrependDecorators(
			decor.Name("Generating Table", decor.WC{C: decor.DindentRight | decor.DextraSpace}),
			decor.OnComplete(
				decor.Any(func(st decor.Statistics) string {
					elapsedStr, _ := elapsedDec.Decor(st)
					etaStr, _ := etaDec.Decor(st)
					return fmt.Sprintf("[%s : %s]", elapsedStr, etaStr)
				}), "done",
			),
		),
		mpb.AppendDecorators(decor.CountersNoUnit("%d / %d"),
			decor.AverageSpeed(0, " %.2f it/s", decor.WC{W: 10}),
		),
	)
	done := make(chan bool)
	go collectResults(results, done, countChan, isVerbose, bar, file, chainsNumber)

	for i := 0; i < chainsNumber; i++ {
		jobs <- seed(i, passwordLength, charset)
	}
	close(jobs)

	wg.Wait()
	close(results)
	finalCount := <-countChan
	<-done

	header.NumChains = finalCount
	file.Seek(0, io.SeekStart)
	binary.Write(file, binary.BigEndian, header)

	printIfVerbose(isVerbose, "Chains Generated\n")
	printIfVerbose(isVerbose, "Table saved to %s\n", path)
}

func collectResults(results <-chan TableEntry, done chan bool, countChan chan uint64, isVerbose bool, bar *mpb.Bar, file *os.File, chainsNumber int) {
	endHashes := make(map[[32]byte]struct{}, chainsNumber)
	uniqueCount := uint64(0)
	updateBar := 0

	for entry := range results {
		if _, exists := endHashes[entry.End]; !exists {
			endHashes[entry.End] = struct{}{}

			file.Write([]byte(entry.Start))
			file.Write(entry.End[:])

			uniqueCount++
		}
		if updateBar%1000 == 0 {
			bar.IncrBy(1000)
		}
		updateBar++
	}
	printIfVerbose(isVerbose, "Generation complete. Collisions: %d\n", chainsNumber-int(uniqueCount))
	countChan <- uniqueCount
	close(countChan)
	done <- true
}
