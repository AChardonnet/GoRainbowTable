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

type Setting struct {
	Name        string
	DisplayName string
	Value       interface{}
	Type        string // "int", "string", "bool"
}

func getSettingValue(settings []Setting, name string) interface{} {
	for _, s := range settings {
		if s.Name == name {
			return s.Value
		}
	}
	return nil
}

func setSettingValue(settings []Setting, name string, value interface{}) []Setting {
	for i, s := range settings {
		if s.Name == name {
			settings[i].Value = value
			break
		}
	}
	return settings
}

func tui() {
	stay := true

	//default Settings
	settings := []Setting{
		{Name: "workerNumber", DisplayName: "Workers", Value: 20, Type: "int"},
		{Name: "chainLength", DisplayName: "Chain Length", Value: 5000, Type: "int"},
		{Name: "passwordLength", DisplayName: "Password Length", Value: 6, Type: "int"},
		{Name: "chainsNumber", DisplayName: "Chains Number", Value: 10000000, Type: "int"},
		{Name: "charset", DisplayName: "Charset", Value: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-", Type: "string"},
		{Name: "tableAutoSelect", DisplayName: "Table Auto Select", Value: true, Type: "bool"},
		{Name: "tableAutoSort", DisplayName: "Table Auto Sort", Value: true, Type: "bool"},
		{Name: "tableAutoGenerate", DisplayName: "Table Auto Generate", Value: true, Type: "bool"},
		{Name: "sortingChunkSize", DisplayName: "Sorting Chunk Size", Value: 2000000, Type: "int"},
	}

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
			computeTable(settings)
		case 2:
			searchPassword(tablesDir, getSettingValue(settings, "workerNumber").(int), getSettingValue(settings, "tableAutoSelect").(bool))
		case 3:
			var err error
			settings, err = settingsMenu(settings)
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

func computeTable(settings []Setting) {
	stay := true
	for stay {
		workerNumber := getSettingValue(settings, "workerNumber").(int)
		chainLength := getSettingValue(settings, "chainLength").(int)
		passwordLength := getSettingValue(settings, "passwordLength").(int)
		charset := getSettingValue(settings, "charset").(string)
		chainsNumber := getSettingValue(settings, "chainsNumber").(int)
		sortingChunkSize := getSettingValue(settings, "sortingChunkSize").(int)
		tableAutoSort := getSettingValue(settings, "tableAutoSort").(bool)

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
			settings, err = settingsMenu(settings)
			if err != nil {
				log.Fatal(err)
			}
		case 2:
			stay = false
		}
	}

}

func settingsMenu(settings []Setting) ([]Setting, error) {
	for {
		// Generate menu options from settings
		options := make([]string, len(settings)+1)
		for i, setting := range settings {
			var valueStr string
			switch setting.Type {
			case "int":
				valueStr = fmt.Sprintf("%d", setting.Value.(int))
			case "string":
				valueStr = setting.Value.(string)
			case "bool":
				valueStr = fmt.Sprintf("%t", setting.Value.(bool))
			}
			options[i] = fmt.Sprintf("%s: %s", setting.DisplayName, valueStr)
		}
		options[len(settings)] = "Back to Main Menu"

		menu := promptui.Select{
			Label: "Select a setting to modify",
			Items: options,
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
			Size: len(options),
		}

		index, _, err := menu.Run()
		if err != nil {
			return settings, err
		}

		// Handle "Back" option
		if index == len(settings) {
			break
		}

		// Update the setting
		settings, err = updateSetting(settings, index)
		if err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func updateSetting(settings []Setting, index int) ([]Setting, error) {
	setting := &settings[index]

	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Enter new value for %s", setting.DisplayName),
		Validate: func(input string) error {
			switch setting.Type {
			case "int":
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("please enter a valid number")
				}
			case "string":
				if len(input) == 0 {
					return fmt.Errorf("input cannot be empty")
				}
			case "bool":
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
		return settings, err
	}

	// Update the setting value based on type
	switch setting.Type {
	case "int":
		setting.Value, _ = strconv.Atoi(result)
	case "string":
		if setting.Name == "charset" {
			setting.Value = generateCharset(result)
		} else {
			setting.Value = result
		}
	case "bool":
		lower := strings.ToLower(result)
		setting.Value = lower == "true" || lower == "yes"
	}

	return settings, nil
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
		selectedTable, found := selectTable(tablesDir, passwordLength, 1, tableAutoSelect)
		if found {
			searchPasswordKnownLength([32]byte(hash), selectedTable, workerNumber)
		} else {
			fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)("No Table found for password Length"))
		}
	case 1:
		for i := 1; i < 6; i++ {
			availableTables, _ := selectTable(tablesDir, i, 1, tableAutoSelect)
			if len(availableTables) > 0 {
				_, found := searchPasswordKnownLength([32]byte(hash), availableTables, workerNumber)
				if found {
					break
				}
			}
		}
	}
}

func selectTable(directory string, passwordLength int, sorted int, autoSelect bool) (path string, found bool) {
	tables, _ := listTables(directory, sorted, passwordLength)

	if len(tables) == 0 {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGBold, promptui.FGRed)(fmt.Sprintf("No matching tables found (Sorted Filter: %d, PassLen Filter: %d)", sorted, passwordLength)))
		return "", false
	}

	wDir, _ := os.Getwd()
	if autoSelect {
		fmt.Println(tables)
		path, _ := filepath.Rel(wDir, filepath.Join(directory, tables[0]))
		return path, true
	} else {
		prompt := promptui.Select{
			Label: "Select a Table",
			Items: tables,
			Size:  10,
		}

		_, result, _ := prompt.Run()
		path, _ := filepath.Rel(wDir, filepath.Join(directory, result))
		return path, true
	}
}
