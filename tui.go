package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func listTables(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var tables []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(directory, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		var header FileHeader
		err = binary.Read(file, binary.BigEndian, &header)
		file.Close()

		if err != nil {
			continue
		}

		if string(header.Magic[:4]) == "RBOW" {
			tables = append(tables, entry.Name())
		}
	}
	return tables, nil
}

func printListTables(directory string) {
	tables, _ := listTables(directory)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "N\tNAME\tVERSION\tPASSWORD_LENGTH\tCHAIN_LENGTH\tCHAIN_COUNT\tCHARSET\tSORTED")
	for i, table := range tables {
		header, charset, _ := readTableHeader(filepath.Join(directory, table))
		sorted := "false"
		if header.IsSorted != 0 {
			sorted = "true"
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%d\t%d\t%s\t%s\n", i, table, header.Version, header.PasswordLength, header.ChainLength, header.NumChains, charset, sorted)
	}
	w.Flush()
}

func menuTables(directory string) {
	// tables, _ := listTables(directory)
}
