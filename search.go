package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

func searchChain(targetHash [32]byte, chain TableEntry, chainLength int, passwordLength int, charset string) (string, bool) {
	currentPlain := string(chain.Start[:])
	var currentHash [32]byte

	for i := 0; i < chainLength; i++ {
		currentHash = hash(currentPlain)

		if currentHash == targetHash {
			return currentPlain, true
		}

		if i < chainLength-1 {
			currentPlain = reduce(currentHash, i, passwordLength, charset)
		}
	}
	return "", false
}

func searchTable(targetHash [32]byte, table []TableEntry, chainLength int, passwordLength int, charset string) (string, bool) {
	// uses binary search : MUST be used with SORTED TABLES (by End)
	for column := chainLength - 1; column >= 0; column-- {
		currentHash := targetHash
		for i := column; i < chainLength-1; i++ {
			plain := reduce(currentHash, i, passwordLength, charset)
			currentHash = hash(plain)
		}

		index := sort.Search(len(table), func(i int) bool {
			return bytes.Compare(table[i].End[:], currentHash[:]) >= 0
		})

		if index < len(table) && table[index].End == currentHash {
			password, found := searchChain(targetHash, table[index], chainLength, passwordLength, charset)
			if found {
				return password, true
			}
		}
	}
	return "", false
}

func searchWorker(
	ctx context.Context,
	targetHash [32]byte,
	table []TableEntry,
	columns <-chan int,
	results chan<- string,
	wg *sync.WaitGroup,
	progressBar *mpb.Bar,
	processed *int64,
	chainLength int,
	passwordLength int,
	charset string,
) {
	defer wg.Done()

	for column := range columns {
		select {
		case <-ctx.Done():
			return
		default:
			currentHash := targetHash
			for i := column; i < chainLength-1; i++ {
				plain := reduce(currentHash, i, passwordLength, charset)
				currentHash = hash(plain)
			}

			if progressBar != nil {
				progressBar.Increment()
			}
			atomic.AddInt64(processed, 1)

			index := sort.Search(len(table), func(i int) bool {
				return bytes.Compare(table[i].End[:], currentHash[:]) >= 0
			})

			if index < len(table) && table[index].End == currentHash {
				password, found := searchChain(targetHash, table[index], chainLength, passwordLength, charset)
				if found {
					select {
					case results <- password:
					default:
					}
					return
				}
			}
		}
	}
}

func searchTableParallel(
	targetHash [32]byte, table []TableEntry, workerNumber int, chainLength int, passwordLength int, charset string, progressBar *mpb.Progress,
) (string, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elapsedDec := decor.Elapsed(decor.ET_STYLE_GO)
	etaDec := decor.AverageETA(decor.ET_STYLE_GO)
	searchBar := progressBar.AddBar(
		int64(chainLength),
		mpb.PrependDecorators(
			decor.Name("Searching Table", decor.WC{C: decor.DindentRight | decor.DextraSpace}),
			decor.OnComplete(
				decor.Any(func(st decor.Statistics) string {
					elapsedStr, _ := elapsedDec.Decor(st)
					etaStr, _ := etaDec.Decor(st)
					return "[" + elapsedStr + " : " + etaStr + "]"
				}),
				"done",
			),
		),
		mpb.AppendDecorators(
			decor.CountersNoUnit("%d / %d "),
			decor.Percentage(),
		),
		mpb.BarRemoveOnComplete(),
	)

	columns := make(chan int, chainLength)
	results := make(chan string, 1)
	var wg sync.WaitGroup
	var processed int64

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go searchWorker(ctx, targetHash, table, columns, results, &wg, searchBar, &processed, chainLength, passwordLength, charset)
	}

	go func() {
		for col := chainLength - 1; col >= 0; col-- {
			columns <- col
		}
		close(columns)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case password := <-results:
		cancel()
		if searchBar != nil {
			searchBar.SetTotal(atomic.LoadInt64(&processed), true)
		}
		wg.Wait()
		return password, true
	case <-done:
		if searchBar != nil {
			searchBar.SetTotal(int64(chainLength), true)
		}
		wg.Wait()
		return "", false
	}
}

func readEntryAt(file *os.File, dataOffset int64, entrySize int64, passwordLength int, index int64) (TableEntry, error) {
	entry := TableEntry{Start: make([]byte, passwordLength)}
	entryOffset := dataOffset + index*entrySize

	if _, err := file.ReadAt(entry.Start, entryOffset); err != nil {
		return TableEntry{}, err
	}

	if _, err := file.ReadAt(entry.End[:], entryOffset+int64(passwordLength)); err != nil {
		return TableEntry{}, err
	}

	return entry, nil
}

func findFirstMatchingEndIndex(file *os.File, dataOffset int64, numChains int64, passwordLength int, targetHash [32]byte) (int64, bool, error) {
	entrySize := int64(passwordLength + len(targetHash))
	left, right := int64(0), numChains

	for left < right {
		mid := left + (right-left)/2
		entry, err := readEntryAt(file, dataOffset, entrySize, passwordLength, mid)
		if err != nil {
			return 0, false, err
		}

		if bytes.Compare(entry.End[:], targetHash[:]) >= 0 {
			right = mid
		} else {
			left = mid + 1
		}
	}

	if left >= numChains {
		return 0, false, nil
	}

	entry, err := readEntryAt(file, dataOffset, entrySize, passwordLength, left)
	if err != nil {
		return 0, false, err
	}

	if entry.End != targetHash {
		return 0, false, nil
	}

	return left, true, nil
}

func searchWorkerOnDisk(
	ctx context.Context,
	targetHash [32]byte,
	file *os.File,
	dataOffset int64,
	numChains int64,
	columns <-chan int,
	results chan<- string,
	wg *sync.WaitGroup,
	progressBar *mpb.Bar,
	processed *int64,
	chainLength int,
	passwordLength int,
	charset string,
) {
	defer wg.Done()

	entrySize := int64(passwordLength + len(targetHash))

	for column := range columns {
		select {
		case <-ctx.Done():
			return
		default:
			currentHash := targetHash
			for i := column; i < chainLength-1; i++ {
				plain := reduce(currentHash, i, passwordLength, charset)
				currentHash = hash(plain)
			}

			if progressBar != nil {
				progressBar.Increment()
			}
			atomic.AddInt64(processed, 1)

			firstIdx, found, err := findFirstMatchingEndIndex(file, dataOffset, numChains, passwordLength, currentHash)
			if err != nil {
				return
			}
			if !found {
				continue
			}

			for idx := firstIdx; idx < numChains; idx++ {
				entry, err := readEntryAt(file, dataOffset, entrySize, passwordLength, idx)
				if err != nil {
					if err == io.EOF {
						break
					}
					return
				}
				if entry.End != currentHash {
					break
				}

				password, chainFound := searchChain(targetHash, entry, chainLength, passwordLength, charset)
				if chainFound {
					select {
					case results <- password:
					default:
					}
					return
				}
			}
		}
	}
}

func searchTableParallelFromDisk(
	targetHash [32]byte,
	tablePath string,
	workerNumber int,
	chainLength int,
	passwordLength int,
	charset string,
	numChains uint64,
	progressBar *mpb.Progress,
) (string, bool, error) {
	file, err := os.Open(tablePath)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	dataOffset := int64(binaryTableDataOffset(len(charset)))
	if numChains == 0 {
		return "", false, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var searchBar *mpb.Bar
	if progressBar != nil {
		elapsedDec := decor.Elapsed(decor.ET_STYLE_GO)
		etaDec := decor.AverageETA(decor.ET_STYLE_GO)
		searchBar = progressBar.AddBar(
			int64(chainLength),
			mpb.PrependDecorators(
				decor.Name("Searching Table", decor.WC{C: decor.DindentRight | decor.DextraSpace}),
				decor.OnComplete(
					decor.Any(func(st decor.Statistics) string {
						elapsedStr, _ := elapsedDec.Decor(st)
						etaStr, _ := etaDec.Decor(st)
						return "[" + elapsedStr + " : " + etaStr + "]"
					}),
					"done",
				),
			),
			mpb.AppendDecorators(
				decor.CountersNoUnit("%d / %d "),
				decor.Percentage(),
			),
			mpb.BarRemoveOnComplete(),
		)
	}

	columns := make(chan int, chainLength)
	results := make(chan string, 1)
	var wg sync.WaitGroup
	var processed int64

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go searchWorkerOnDisk(
			ctx,
			targetHash,
			file,
			dataOffset,
			int64(numChains),
			columns,
			results,
			&wg,
			searchBar,
			&processed,
			chainLength,
			passwordLength,
			charset,
		)
	}

	go func() {
		for col := chainLength - 1; col >= 0; col-- {
			columns <- col
		}
		close(columns)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case password := <-results:
		cancel()
		if searchBar != nil {
			searchBar.SetTotal(atomic.LoadInt64(&processed), true)
		}
		wg.Wait()
		return password, true, nil
	case <-done:
		if searchBar != nil {
			searchBar.SetTotal(int64(chainLength), true)
		}
		wg.Wait()
		return "", false, nil
	}
}
