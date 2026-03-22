package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
)

func tui() {
	stay := true

	//default Settings
	var (
		charset          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
		passwordLength   = 6
		chainLength      = 5000
		chainsNumber     = 10000000
		workerNumber     = 20
		tableAutoSelect  = true
		tableAutoSort    = true
		sortingChunkSize = 2000000
	)

	dir, _ := os.Getwd()
	tablesDir := filepath.Join(dir, "tables")
	for stay {
		items := []string{"List Tables", "Compute Table", "Search Table", "Settings", "Exit"}

		prompt := promptui.Select{
			Label: "Select Action",
			Items: items,
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
		}

		index, _, err := prompt.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		switch index {
		case 0:
			printListTables(tablesDir)
		case 1:
			computeTable(workerNumber, chainLength, passwordLength, charset, chainsNumber, sortingChunkSize, tableAutoSort)
		case 2:
			searchPassword(tablesDir, workerNumber, tableAutoSelect)
		case 3:
			err, workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect = settingsMenu(workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect)
			if err != nil {
				fmt.Printf("Settings menu error: %v\n", err)
			}
		case 4:
			stay = false
		}
	}
}

func listTables(directory string, filterSorted int, filterPassLen int) ([]string, error) {
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

		if string(header.Magic[:4]) != "RBOW" {
			continue
		}
		if filterSorted != -1 {
			if uint8(filterSorted) != header.IsSorted {
				continue
			}
		}
		if filterPassLen != -1 {
			if uint32(filterPassLen) != header.PasswordLength {
				continue
			}
		}
		tables = append(tables, entry.Name())
	}
	return tables, nil
}

func printListTables(directory string) {
	tables, _ := listTables(directory, -1, -1)
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

func computeTable(workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int, sortingChunkSize int, tableAutoSort bool) {
	stay := true
	for stay {
		fmt.Println(promptui.Styler(promptui.FGBold)("\n--- Rainbow Table Configuration ---"))
		fmt.Printf("  %s %d\n", promptui.Styler(promptui.FGBlue)("Workers:"), workerNumber)
		fmt.Printf("  %s %d\n", promptui.Styler(promptui.FGBlue)("Chain Length:"), chainLength)
		fmt.Printf("  %s %d\n", promptui.Styler(promptui.FGBlue)("Password Length:"), passwordLength)
		fmt.Printf("  %s %s\n", promptui.Styler(promptui.FGBlue)("Charset:"), charset)
		fmt.Printf("  %s %d\n", promptui.Styler(promptui.FGBlue)("Table Length:"), chainsNumber)
		probaSuccess := calculateSuccessProbability(chainsNumber, chainLength, passwordLength, charset)
		fmt.Printf("  %s %.2f%%\n", promptui.Styler(promptui.FGGreen)("Success Probability:"), probaSuccess*100)
		prompt := promptui.Select{
			Label: "Compute table?",
			Items: []string{"Yes, Start Computing", "No, Change Settings", "Exit"},
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
		}
		index, _, _ := prompt.Run()
		switch index {
		case 0:
			var index int
			path := generateTableMultiThread(workerNumber, chainLength, passwordLength, charset, chainsNumber)
			if tableAutoSort {
				index = 0
			} else {
				prompt := promptui.Select{
					Label: "Sort table?",
					Items: []string{"Yes", "No"},
					Templates: &promptui.SelectTemplates{
						Selected: "{{ . | green }}",
					},
				}
				index, _, _ = prompt.Run()
			}
			switch index {
			case 0:
				SortLargeTable(path, sortingChunkSize)
			case 1:
				continue
			}
		case 1:
			var err error
			err, workerNumber, chainLength, passwordLength, charset, chainsNumber, _ = settingsMenu(workerNumber, chainLength, passwordLength, charset, chainsNumber, true)
			if err != nil {
				log.Fatal(err)
			}
		case 2:
			stay = false
		}
	}

}

func settingsMenu(workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int, tableAutoSelect bool) (error, int, int, int, string, int, bool) {
	for {
		// 1. Define the labels with current values for the menu
		options := []string{
			fmt.Sprintf("Workers: %d", workerNumber),
			fmt.Sprintf("Chain Length: %d", chainLength),
			fmt.Sprintf("Password Length: %d", passwordLength),
			fmt.Sprintf("Chains Number: %d", chainsNumber),
			fmt.Sprintf("Charset: %s", charset),
			fmt.Sprintf("Table Auto Select: %t", tableAutoSelect),
			"Back to Main Menu",
		}

		menu := promptui.Select{
			Label: "Select a setting to modify",
			Items: options,
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
			Size: 7,
		}

		index, _, err := menu.Run()
		if err != nil {
			return err, 0, 0, 0, "", 0, false
		}

		// Handle "Back" option
		if index == 6 {
			break
		}

		// 2. Prompt for the new value
		err, workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect = updateSetting(
			index, workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect,
		)
	}
	return nil, workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect
}

func updateSetting(index int, workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int, tableAutoSelect bool) (error, int, int, int, string, int, bool) {
	labels := []string{"Workers", "Chain Length", "Password Length", "Chains Number", "Charset", "Table Auto Select"}

	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Enter new value for %s", labels[index]),
		// Basic validation to ensure numbers are numbers
		Validate: func(input string) error {
			if index < 4 { // Indices 0-3 are integers
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("please enter a valid number")
				}
			} else if index == 4 { // Charset
				if len(input) == 0 {
					return fmt.Errorf("input cannot be empty")
				}
			} else if index == 5 { // Table Auto Select
				lower := strings.ToLower(input)
				if lower != "true" && lower != "false" && lower != "yes" && lower != "no" {
					return fmt.Errorf("please enter true, false, yes, or no")
				}
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return err, 0, 0, 0, "", 0, false
	}

	// 3. Update the global/struct variables
	switch index {
	case 0:
		workerNumber, _ = strconv.Atoi(result)
	case 1:
		chainLength, _ = strconv.Atoi(result)
	case 2:
		passwordLength, _ = strconv.Atoi(result)
	case 3:
		chainsNumber, _ = strconv.Atoi(result)
	case 4:
		charset = generateCharset(result)
	case 5:
		lower := strings.ToLower(result)
		tableAutoSelect = lower == "true" || lower == "yes"
	}
	return nil, workerNumber, chainLength, passwordLength, charset, chainsNumber, tableAutoSelect
}

func searchPasswordKnownLength(targetHash [32]byte, tablePath string, workerNumber int) (string, bool) {
	header, charset, table, _ := loadTableWithHeader(tablePath)

	if header.IsSorted == 0 {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGBold, promptui.FGRed)("THIS TABLE IS NOT SORTED THE SEARCH WON'T WORK"))
	}

	password, found := searchTableParallel(targetHash, table, workerNumber, int(header.ChainLength), int(header.PasswordLength), charset)

	if found {
		fmt.Printf("  %s %s\n", promptui.Styler(promptui.FGGreen)("Password Found"), password)
	} else {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)("Password Not Found"))
	}
	return password, found
}

func searchPassword(tablesDir string, workerNumber int, tableAutoSelect bool) {
	validate := func(input string) error {
		// This regex checks if it's a hex string (0-9, a-f)
		// Adjust the length {32,64} based on the hash type you expect
		match, _ := regexp.MatchString("^[a-fA-F0-9]+$", input)
		if !match {
			return errors.New("invalid format: please enter a hex hash")
		} else {
			if len(input) != 64 {
				return errors.New("must be 64 characters")
			}
			return nil
		}
	}

	promptBool := promptui.Select{
		Label: "Password Length Known?",
		Items: []string{"Yes", "No"},
		Templates: &promptui.SelectTemplates{
			Selected: "{{ . | green }}",
		},
	}

	validateNumber := func(input string) error {
		_, err := strconv.Atoi(input)
		if err != nil {
			return errors.New("please enter a valid number")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Enter Hash",
		Validate: validate,
	}

	result, err := prompt.Run()
	fmt.Println(result)
	result = strings.ToLower(result)
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "0x")

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	hash, err := hex.DecodeString(result)
	if err != nil {
		fmt.Printf("Failed to parse hash: %v\n", err)
		return
	}

	promptNumber := promptui.Prompt{
		Label:    "password length",
		Validate: validateNumber,
	}

	index, _, err := promptBool.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	switch index {
	case 0:
		result, err := promptNumber.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		passwordLength, err := strconv.Atoi(result)
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		searchPasswordKnownLength([32]byte(hash), selectTable(tablesDir, passwordLength, 1, tableAutoSelect), workerNumber)
	case 1:
		for i := 0; i < 5; i++ {
			availableTables := selectTable(tablesDir, i, 1, tableAutoSelect)
			if len(availableTables) > 0 {
				_, found := searchPasswordKnownLength([32]byte(hash), selectTable(tablesDir, i, 1, tableAutoSelect), workerNumber)
				if found {
					break
				}
			} else {
				fmt.Printf(" %s %d\n", promptui.Styler(promptui.FGBold, promptui.FGRed)("No Table found for password Length"), i)
			}
		}
	}
}

func selectTable(directory string, passwordLength int, sorted int, autoSelect bool) string {
	tables, _ := listTables(directory, sorted, passwordLength)
	wDir, _ := os.Getwd()
	if autoSelect {
		fmt.Println(tables)
		path, _ := filepath.Rel(wDir, filepath.Join(directory, tables[0]))
		return path
	} else {
		prompt := promptui.Select{
			Label: "Select a Table",
			Items: tables,
			Size:  10,
		}

		_, result, _ := prompt.Run()
		path, _ := filepath.Rel(wDir, filepath.Join(directory, result))
		return path
	}
}
