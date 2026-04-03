package main

import (
	"bufio"
	"bytes"
	"container/heap"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type MergeItem struct {
	Entry     TableEntry
	FileIndex int
}

type MergeHeap []MergeItem

func (h MergeHeap) Len() int {
	return len(h)
}

func (h MergeHeap) Less(i, j int) bool {
	return bytes.Compare(h[i].Entry.End[:], h[j].Entry.End[:]) < 0
}

func (h MergeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *MergeHeap) Push(x any) {
	*h = append(*h, x.(MergeItem))
}

func (h *MergeHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func SortLargeTable(inputPath string, chunkSize int, progressBar *mpb.Progress, tableDisplayName string, numWorkers int) error {
	fileName := filepath.Base(inputPath)
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	tmpDir := filepath.Join(workDir, "tmp")

	header, charset, err := readTableHeader(inputPath)
	if err != nil {
		return err
	}
	elapsedDec := decor.Elapsed(decor.ET_STYLE_GO)
	etaDec := decor.AverageETA(decor.ET_STYLE_GO)
	mainBar := progressBar.AddBar(
		int64(4*header.NumChains),
		mpb.PrependDecorators(
			decor.Name("Sorting Table "+tableDisplayName, decor.WC{C: decor.DindentRight | decor.DextraSpace}), decor.OnComplete(
				decor.Any(func(st decor.Statistics) string {
					elapsedStr, _ := elapsedDec.Decor(st)
					etaStr, _ := etaDec.Decor(st)
					return fmt.Sprintf("[%s : %s]", elapsedStr, etaStr)
				}), "done",
			),
		),
		mpb.AppendDecorators(decor.Percentage()),
	)

	if chunkSize > int(header.NumChains) {
		chunkSize = int(header.NumChains)
	}

	file, _ := os.Open(inputPath)
	defer file.Close()

	// Skip header and charset to get to data
	headerSize := uint32(binary.Size(FileHeader{}))
	paddingSize := (8 - ((headerSize + uint32(len(charset))) % 8)) % 8
	file.Seek(int64(headerSize+uint32(len(charset))+paddingSize), 0)

	var tempFiles []string
	var tempFilesMutex sync.Mutex

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}

	// --- PHASE 1: SPLIT AND SORT CHUNKS (with concurrent sorting) ---
	type ChunkJob struct {
		ID   int
		Data []TableEntry
		Path string
		Bars struct {
			Sort  *mpb.Bar
			Save  *mpb.Bar
			Chunk *mpb.Bar
		}
	}

	chunkChan := make(chan *ChunkJob, numWorkers*10) // Buffer channel for concurrent processing
	var wg sync.WaitGroup

	// Start worker goroutines for sorting and saving chunks
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range chunkChan {
				// Sort chunk in RAM
				sort.Slice(job.Data, func(i, j int) bool {
					return bytes.Compare(job.Data[i].End[:], job.Data[j].End[:]) < 0
				})
				job.Bars.Sort.SetTotal(1, true)

				// Save temp file with buffered writer
				tempFile, _ := os.Create(job.Path)
				bufWriter := bufio.NewWriterSize(tempFile, 256*1024) // 256KB buffer
				progressUpdate := 0
				for _, entry := range job.Data {
					bufWriter.Write(entry.Start)
					bufWriter.Write(entry.End[:])
					if progressUpdate%1000 == 0 {
						job.Bars.Save.IncrBy(1000)
						job.Bars.Chunk.IncrBy(1000)
						mainBar.IncrBy(1000)
					}
					progressUpdate++
				}
				bufWriter.Flush()
				tempFile.Sync()
				tempFile.Close()

				job.Bars.Save.IncrBy(progressUpdate % 1000)
				job.Bars.Chunk.IncrBy(progressUpdate % 1000)
				mainBar.IncrBy(progressUpdate % 1000)
			}
		}()
	}

	usedChains := 0
	chunkId := 0
	for {
		firstEntry := TableEntry{Start: make([]byte, header.PasswordLength)}
		if _, err := io.ReadFull(file, firstEntry.Start); err != nil {
			break // No more data, exit loop safely
		}
		if _, err := io.ReadFull(file, firstEntry.End[:]); err != nil {
			break
		}
		remaining := int(header.NumChains) - (chunkId * chunkSize)
		currentBarTotal := chunkSize
		if remaining < chunkSize {
			currentBarTotal = remaining
		}
		chunkBar := progressBar.AddBar(int64(2*currentBarTotal),
			mpb.PrependDecorators(decor.Name(fmt.Sprintf("Creating Chunk %d", chunkId))),
			mpb.AppendDecorators(decor.Percentage()),
			mpb.BarRemoveOnComplete(),
		)
		readBar := progressBar.AddBar(int64(currentBarTotal),
			mpb.PrependDecorators(decor.Name(fmt.Sprintf("Reading Chunk %d", chunkId))),
			mpb.AppendDecorators(decor.Percentage()),
			mpb.BarRemoveOnComplete(),
		)
		buffer := make([]TableEntry, 0, currentBarTotal)
		buffer = append(buffer, firstEntry)
		mainBar.Increment()
		chunkBar.Increment()
		readBar.Increment()
		usedChains++
		progressUpdate := 0
		for i := 1; i < currentBarTotal; i++ {
			entry := TableEntry{Start: make([]byte, header.PasswordLength)}
			if _, err := io.ReadFull(file, entry.Start); err != nil {
				break
			}
			if _, err := io.ReadFull(file, entry.End[:]); err != nil {
				break
			}
			buffer = append(buffer, entry)
			progressUpdate++
			usedChains++
			if progressUpdate%1000 == 0 {
				mainBar.IncrBy(1000)
				chunkBar.IncrBy(1000)
				readBar.IncrBy(1000)
			}
		}
		mainBar.IncrBy(progressUpdate % 1000)
		chunkBar.IncrBy(progressUpdate % 1000)
		readBar.IncrBy(progressUpdate % 1000)

		// Create job for sorting and saving (enqueue to workers)
		sortBar := progressBar.AddBar(0,
			mpb.PrependDecorators(
				decor.Name(fmt.Sprintf("Sorting Chunk %d ", chunkId)),
				decor.Elapsed(decor.ET_STYLE_GO),
			),
			mpb.AppendDecorators(decor.OnComplete(decor.Spinner(nil), "done!")),
			mpb.BarRemoveOnComplete(),
		)
		saveBar := progressBar.AddBar(int64(currentBarTotal),
			mpb.PrependDecorators(decor.Name(fmt.Sprintf("Saving Chunk %d", chunkId))),
			mpb.AppendDecorators(decor.Percentage()),
			mpb.BarRemoveOnComplete(),
		)
		tempPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%d.tmp", baseName, chunkId))

		job := &ChunkJob{
			ID:   chunkId,
			Data: buffer,
			Path: tempPath,
		}
		job.Bars.Sort = sortBar
		job.Bars.Save = saveBar
		job.Bars.Chunk = chunkBar

		chunkChan <- job
		tempFilesMutex.Lock()
		tempFiles = append(tempFiles, tempPath)
		tempFilesMutex.Unlock()

		chunkId++
	}

	// Close channel and wait for all workers to finish
	close(chunkChan)
	wg.Wait()

	// --- PHASE 2: MERGE CHUNKS ---
	return mergeChunks(header, charset, tempFiles, mainBar, progressBar)
}

func mergeChunks(header FileHeader, charset string, tempPaths []string, mainBar *mpb.Bar, progressBar *mpb.Progress) error {
	mergeBar := progressBar.AddBar(int64(2*header.NumChains),
		mpb.PrependDecorators(decor.Name("Merging Chunks")),
		mpb.AppendDecorators(decor.Percentage()),
		mpb.BarRemoveOnComplete(),
	)
	dir, _ := os.Getwd()
	tablesDir := filepath.Join(dir, "tables")
	creationTime := time.Now().Format("2006-01-02_15-04-05")
	tableName := creationTime + "_" + uuid.New().String()[:8]
	outputPath := filepath.Join(tablesDir, tableName+".rtable")

	out, _ := os.Create(outputPath)
	defer out.Close()

	header.IsSorted = 1
	binary.Write(out, binary.BigEndian, header)
	out.Write([]byte(charset))

	paddingSize := (8 - ((uint32(binary.Size(header)) + uint32(len(charset))) % 8)) % 8
	if paddingSize > 0 {
		out.Write(make([]byte, paddingSize))
	}

	// Open all temp files
	files := make([]*os.File, len(tempPaths))
	hp := &MergeHeap{}
	heap.Init(hp)

	for i, path := range tempPaths {
		f, _ := os.Open(path)
		defer f.Close()
		files[i] = f
		// Push first item from each file into heap
		entry := readNext(f, int(header.PasswordLength))
		if entry != nil {
			heap.Push(hp, MergeItem{*entry, i})
		}
	}

	// Use buffered writer for merge phase to reduce I/O calls
	bufWriter := bufio.NewWriterSize(out, 256*1024) // 256KB write buffer

	var lastWrittenHash [32]byte
	var uniqueCount uint64 = 0
	firstEntry := true
	progressUpdate := 0
	for hp.Len() > 0 {
		// Get smallest End hash
		minItem := heap.Pop(hp).(MergeItem)
		currentEntry := minItem.Entry

		// Deduplication Check
		if firstEntry || !bytes.Equal(currentEntry.End[:], lastWrittenHash[:]) {
			bufWriter.Write(currentEntry.Start)
			bufWriter.Write(currentEntry.End[:])

			lastWrittenHash = currentEntry.End
			uniqueCount++
			firstEntry = false
		}

		// Read next item from the file we just pulled from
		next := readNext(files[minItem.FileIndex], int(header.PasswordLength))
		if next != nil {
			heap.Push(hp, MergeItem{*next, minItem.FileIndex})
		}
		if progressUpdate%1000 == 0 {
			mainBar.IncrBy(2000)
			mergeBar.IncrBy(2000)
		}
		progressUpdate++
	}
	mainBar.IncrBy(2 * (progressUpdate % 1000))
	mergeBar.IncrBy(2 * (progressUpdate % 1000))

	// Flush buffer to disk before updating header
	bufWriter.Flush()

	header.NumChains = uniqueCount

	out.Seek(0, io.SeekStart)
	binary.Write(out, binary.BigEndian, header)

	out.Sync()

	for _, f := range files {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}
	for _, p := range tempPaths {
		err := os.Remove(p)
		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

func readNext(f *os.File, pLen int) *TableEntry {
	entry := &TableEntry{Start: make([]byte, pLen)}
	if _, err := io.ReadFull(f, entry.Start); err != nil {
		return nil
	}
	if _, err := io.ReadFull(f, entry.End[:]); err != nil {
		return nil
	}
	return entry
}
