package main

import (
	"bytes"
	"context"
	"sort"
	"sync"
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
		for i := column; i < chainLength; i++ {
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
			for i := column; i < chainLength; i++ {
				plain := reduce(currentHash, i, passwordLength, charset)
				currentHash = hash(plain)
			}

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
	targetHash [32]byte, table []TableEntry, workerNumber int, chainLength int, passwordLength int, charset string,
) (string, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	columns := make(chan int, chainLength)
	results := make(chan string, 1)
	var wg sync.WaitGroup

	for w := 0; w < workerNumber; w++ {
		wg.Add(1)
		go searchWorker(ctx, targetHash, table, columns, results, &wg, chainLength, passwordLength, charset)
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
		return password, true
	case <-done:
		return "", false
	}
}
