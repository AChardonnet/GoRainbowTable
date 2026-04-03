package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
	"github.com/vbauerster/mpb/v8"
)

type Setting struct {
	Name        string
	DisplayName string
	Value       interface{}
	Type        string // "int", "string", "bool"
}

type SettingsData struct {
	WorkerNumber             int    `json:"workerNumber"`
	ChainLength              int    `json:"chainLength"`
	PasswordLength           int    `json:"passwordLength"`
	ChainsNumber             int    `json:"chainsNumber"`
	Charset                  string `json:"charset"`
	TableAutoSelect          bool   `json:"tableAutoSelect"`
	TableAutoSort            bool   `json:"tableAutoSort"`
	AutoRemoveUnsortedTables bool   `json:"autoRemoveUnsortedTables"`
	TableAutoGenerate        bool   `json:"tableAutoGenerate"`
	SortingChunkSize         int    `json:"sortingChunkSize"`
}

func settingsFilePath() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(workDir, ".GoRainbowTable", "settings.json"), nil
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

func settingsToData(settings []Setting) SettingsData {
	return SettingsData{
		WorkerNumber:             getSettingValue(settings, "workerNumber").(int),
		ChainLength:              getSettingValue(settings, "chainLength").(int),
		PasswordLength:           getSettingValue(settings, "passwordLength").(int),
		ChainsNumber:             getSettingValue(settings, "chainsNumber").(int),
		Charset:                  getSettingValue(settings, "charset").(string),
		TableAutoSelect:          getSettingValue(settings, "tableAutoSelect").(bool),
		TableAutoSort:            getSettingValue(settings, "tableAutoSort").(bool),
		AutoRemoveUnsortedTables: getSettingValue(settings, "autoRemoveUnsortedTables").(bool),
		TableAutoGenerate:        getSettingValue(settings, "tableAutoGenerate").(bool),
		SortingChunkSize:         getSettingValue(settings, "sortingChunkSize").(int),
	}
}

func dataToSettings(data SettingsData) []Setting {
	return []Setting{
		{Name: "workerNumber", DisplayName: "Workers", Value: data.WorkerNumber, Type: "int"},
		{Name: "chainLength", DisplayName: "Chain Length", Value: data.ChainLength, Type: "int"},
		{Name: "passwordLength", DisplayName: "Password Length", Value: data.PasswordLength, Type: "int"},
		{Name: "chainsNumber", DisplayName: "Chains Number", Value: data.ChainsNumber, Type: "int"},
		{Name: "charset", DisplayName: "Charset", Value: data.Charset, Type: "string"},
		{Name: "tableAutoSelect", DisplayName: "Table Auto Select", Value: data.TableAutoSelect, Type: "bool"},
		{Name: "tableAutoSort", DisplayName: "Table Auto Sort", Value: data.TableAutoSort, Type: "bool"},
		{Name: "autoRemoveUnsortedTables", DisplayName: "Auto Remove Unsorted Tables", Value: data.AutoRemoveUnsortedTables, Type: "bool"},
		{Name: "tableAutoGenerate", DisplayName: "Table Auto Generate", Value: data.TableAutoGenerate, Type: "bool"},
		{Name: "sortingChunkSize", DisplayName: "Sorting Chunk Size", Value: data.SortingChunkSize, Type: "int"},
	}
}

func saveSettings(settings []Setting) error {
	data := settingsToData(settings)
	settingsPath, err := settingsFilePath()
	if err != nil {
		return fmt.Errorf("failed to resolve settings path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	file, err := os.Create(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to create settings file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode settings: %w", err)
	}

	return nil
}

func loadSettings() ([]Setting, error) {
	settingsPath, err := settingsFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve settings path: %w", err)
	}

	file, err := os.Open(settingsPath)
	if err != nil {
		// If file doesn't exist, return default settings
		if os.IsNotExist(err) {
			return getDefaultSettings(), nil
		}
		return nil, fmt.Errorf("failed to open settings file: %w", err)
	}
	defer file.Close()

	var data SettingsData
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode settings: %w", err)
	}

	return dataToSettings(data), nil
}

func getDefaultSettings() []Setting {
	return []Setting{
		{Name: "workerNumber", DisplayName: "Workers", Value: 20, Type: "int"},
		{Name: "chainLength", DisplayName: "Chain Length", Value: 10000, Type: "int"},
		{Name: "passwordLength", DisplayName: "Password Length", Value: 6, Type: "int"},
		{Name: "chainsNumber", DisplayName: "Chains Number", Value: 10000000, Type: "int"},
		{Name: "charset", DisplayName: "Charset", Value: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-", Type: "string"},
		{Name: "tableAutoSelect", DisplayName: "Table Auto Select", Value: true, Type: "bool"},
		{Name: "tableAutoSort", DisplayName: "Table Auto Sort", Value: true, Type: "bool"},
		{Name: "autoRemoveUnsortedTables", DisplayName: "Auto Remove Unsorted Tables", Value: false, Type: "bool"},
		{Name: "tableAutoGenerate", DisplayName: "Table Auto Generate", Value: true, Type: "bool"},
		{Name: "sortingChunkSize", DisplayName: "Sorting Chunk Size", Value: 2000000, Type: "int"},
	}
}

func tui() {
	stay := true
	//default Settings
	settings, err := loadSettings()
	if err != nil {
		fmt.Printf("Warning: Failed to load settings, using defaults: %v\n", err)
		settings = getDefaultSettings()
	}

	dir, _ := os.Getwd()
	tablesDir := filepath.Join(dir, "tables")
	for stay {
		progressBar := mpb.New(
			mpb.WithAutoRefresh(),
			mpb.WithOutput(os.Stderr),
		)
		items := []string{"List Tables", "Compute Table", "Compute Multiple Tables", "Search Table", "PruneTable", "Sort Table", "Settings", "Exit"}

		prompt := promptui.Select{
			Label: "Select Action",
			Items: items,
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
			Size: 8,
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
			computeTable(settings, progressBar)
			progressBar.Wait()
		case 2:
			computeMultipleTables(settings, progressBar)
			progressBar.Wait()
		case 3:
			searchPassword(tablesDir, getSettingValue(settings, "workerNumber").(int), getSettingValue(settings, "tableAutoSelect").(bool), progressBar)
			progressBar.Wait()
		case 4:
			pruneTablesMenu(tablesDir)
		case 5:
			sortTableMenu(tablesDir, settings, progressBar)
			progressBar.Wait()
		case 6:
			var err error
			settings, err = settingsMenu(settings)
			if err != nil {
				fmt.Printf("Settings menu error: %v\n", err)
			}
		case 7:
			stay = false
		}
	}
}

func pruneTablesMenu(directory string) {
	stay := true
	for stay {
		prompt := promptui.Select{
			Label: "PruneTable",
			Items: []string{"Remove unsorted tables", "Remove duplicate tables", "Back"},
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
			Size: 3,
		}

		index, _, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		switch index {
		case 0:
			removed, err := removeUnsortedTables(directory)
			if err != nil {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Failed to remove unsorted tables: %v", err)))
				continue
			}
			fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Removed %d unsorted tables", removed)))
		case 1:
			removed, err := removeDuplicateTables(directory)
			if err != nil {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Failed to remove duplicate tables: %v", err)))
				continue
			}
			fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Removed %d duplicate tables", removed)))
		case 2:
			stay = false
		}
	}
}

func sortTableMenu(directory string, settings []Setting, progressBar *mpb.Progress) {
	tables, err := listTables(directory, 0, -1, false)
	if err != nil || len(tables) == 0 {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)("No unsorted tables found"))
		return
	}

	prompt := promptui.Select{
		Label: "Select a Table to Sort",
		Items: tables,
		Size:  10,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	wDir, _ := os.Getwd()
	selectedPath, _ := filepath.Rel(wDir, filepath.Join(directory, result))
	chunkSize := getSettingValue(settings, "sortingChunkSize").(int)
	workerNumber := getSettingValue(settings, "workerNumber").(int)

	if err := SortLargeTable(selectedPath, chunkSize, progressBar, result, workerNumber); err != nil {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Failed to sort table: %v", err)))
		return
	}

	fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Table sorted successfully: %s", result)))
}

func removeUnsortedTables(directory string) (int, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(directory, entry.Name())
		header, _, err := readTableHeader(path)
		if err != nil {
			continue
		}

		if string(header.Magic[:4]) != "RBOW" {
			continue
		}

		if header.IsSorted == 0 {
			if err := os.Remove(path); err != nil {
				return removed, fmt.Errorf("failed to remove %s: %w", path, err)
			}
			removed++
		}
	}

	return removed, nil
}

func removeDuplicateTables(directory string) (int, error) {
	type tableMeta struct {
		name           string
		path           string
		passwordLength uint32
		probability    float64
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	groups := make(map[uint32][]tableMeta)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(directory, entry.Name())
		header, charset, err := readTableHeader(path)
		if err != nil {
			continue
		}
		if string(header.Magic[:4]) != "RBOW" {
			continue
		}
		if header.IsSorted == 0 {
			continue
		}

		probability := calculateSuccessProbability(int(header.NumChains), int(header.ChainLength), int(header.PasswordLength), charset)
		meta := tableMeta{
			name:           entry.Name(),
			path:           path,
			passwordLength: header.PasswordLength,
			probability:    probability,
		}
		groups[header.PasswordLength] = append(groups[header.PasswordLength], meta)
	}

	removed := 0
	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}

		sort.Slice(group, func(i, j int) bool {
			if group[i].probability == group[j].probability {
				return group[i].name < group[j].name
			}
			return group[i].probability > group[j].probability
		})

		for i := 1; i < len(group); i++ {
			if err := os.Remove(group[i].path); err != nil {
				return removed, fmt.Errorf("failed to remove duplicate %s: %w", group[i].path, err)
			}
			removed++
		}
	}

	return removed, nil
}

func listTables(directory string, filterSorted int, filterPassLen int, sorted bool) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	type tableCandidate struct {
		name        string
		probability float64
	}

	var tables []tableCandidate
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
		if err != nil {
			file.Close()
			continue
		}

		charsetBytes := make([]byte, header.CharsetLength)
		if _, err := io.ReadFull(file, charsetBytes); err != nil {
			file.Close()
			continue
		}
		file.Close()

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

		charset := string(charsetBytes)
		probability := calculateSuccessProbability(int(header.NumChains), int(header.ChainLength), int(header.PasswordLength), charset)
		if header.IsSorted == 0 {
			probability = 0
		}
		tables = append(tables, tableCandidate{name: entry.Name(), probability: probability})
	}

	if sorted {
		sort.Slice(tables, func(i, j int) bool {
			return tables[i].probability > tables[j].probability
		})
	}

	result := make([]string, 0, len(tables))
	for _, table := range tables {
		result = append(result, table.name)
	}

	return result, nil
}

func printListTables(directory string) {
	tables, _ := listTables(directory, -1, -1, false)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "N\tNAME\tVERSION\tPASSWORD_LENGTH\tCHAIN_LENGTH\tCHAIN_COUNT\tCHARSET\tSORTED")
	for i, table := range tables {
		header, charset, _ := readTableHeader(filepath.Join(directory, table))
		sorted := "false"
		if header.IsSorted != 0 {
			sorted = "true"
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%d\t%d\t%s\t%s\n", i, table, header.Version, header.PasswordLength, header.ChainLength, header.NumChains, displayCharset(charset), sorted)
	}
	w.Flush()
}

func computeTable(settings []Setting, progressBar *mpb.Progress) {
	workerNumber := getSettingValue(settings, "workerNumber").(int)
	chainLength := getSettingValue(settings, "chainLength").(int)
	passwordLength := getSettingValue(settings, "passwordLength").(int)
	charset := getSettingValue(settings, "charset").(string)
	chainsNumber := getSettingValue(settings, "chainsNumber").(int)
	sortingChunkSize := getSettingValue(settings, "sortingChunkSize").(int)
	tableAutoSort := getSettingValue(settings, "tableAutoSort").(bool)
	autoRemoveUnsortedTables := getSettingValue(settings, "autoRemoveUnsortedTables").(bool)

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
	index, _, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}
	switch index {
	case 0:
		var index int
		tableDisplayName := fmt.Sprintf("PL%d_CL%d_TL%d", passwordLength, chainLength, chainsNumber)
		path := generateTableMultiThread(workerNumber, chainLength, passwordLength, charset, chainsNumber, progressBar, tableDisplayName)
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
			index, _, err = prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}
		}
		switch index {
		case 0:
			SortLargeTable(path, sortingChunkSize, progressBar, tableDisplayName, workerNumber)
			if autoRemoveUnsortedTables {
				dir, _ := os.Getwd()
				tablesDir := filepath.Join(dir, "tables")
				removed, err := removeUnsortedTables(tablesDir)
				if err != nil {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Auto remove unsorted tables failed: %v", err)))
				} else if removed > 0 {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Auto removed %d unsorted tables", removed)))
				}
			}
			return
		case 1:
			return
		}
		return
	case 1:
		var err error
		settings, err = settingsMenu(settings)
		if err != nil {
			log.Fatal(err)
		}
		return
	case 2:
		return
	}
}

func computeMultipleTables(settings []Setting, progressBar *mpb.Progress) {
	// Choose configuration mode
	modePrompt := promptui.Select{
		Label: "Configuration Mode",
		Items: []string{"Manual Configuration", "Auto Configuration (by probability)"},
		Templates: &promptui.SelectTemplates{
			Selected: "{{ . | green }}",
		},
	}
	modeIndex, _, err := modePrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	if modeIndex == 0 {
		computeMultipleTablesManual(settings, progressBar)
	} else {
		computeMultipleTablesAuto(settings, progressBar)
	}
}

func computeMultipleTablesManual(settings []Setting, progressBar *mpb.Progress) {
	numTablesPrompt := promptui.Prompt{
		Label: "How many tables do you want to compute?",
		Validate: func(input string) error {
			num, err := strconv.Atoi(input)
			if err != nil {
				return fmt.Errorf("enter a valid number")
			}
			if num < 1 {
				return fmt.Errorf("must be at least 1")
			}
			if num > 10 {
				return fmt.Errorf("maximum 10 tables at once for system stability")
			}
			return nil
		},
	}

	numTablesStr, err := numTablesPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}
	numTables, _ := strconv.Atoi(numTablesStr)

	type TableConfig struct {
		workerNumber   int
		chainLength    int
		passwordLength int
		chainsNumber   int
	}

	configs := make([]TableConfig, numTables)
	baseWorkerNumber := getSettingValue(settings, "workerNumber").(int)
	baseChainLength := getSettingValue(settings, "chainLength").(int)
	basePasswordLength := getSettingValue(settings, "passwordLength").(int)
	baseCharset := getSettingValue(settings, "charset").(string)
	baseChainsNumber := getSettingValue(settings, "chainsNumber").(int)
	sortingChunkSize := getSettingValue(settings, "sortingChunkSize").(int)
	tableAutoSort := getSettingValue(settings, "tableAutoSort").(bool)
	autoRemoveUnsortedTables := getSettingValue(settings, "autoRemoveUnsortedTables").(bool)

	for i := 0; i < numTables; i++ {
		fmt.Printf("\n%s--- Table %d Configuration ---\n", promptui.Styler(promptui.FGBold)(""), i+1)

		passLenPrompt := promptui.Prompt{
			Label:   "Password Length",
			Default: fmt.Sprintf("%d", basePasswordLength),
			Validate: func(input string) error {
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("enter a valid number")
				}
				return nil
			},
		}
		passLenStr, err := passLenPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		passwordLength, _ := strconv.Atoi(passLenStr)

		chainLenPrompt := promptui.Prompt{
			Label:   "Chain Length",
			Default: fmt.Sprintf("%d", baseChainLength),
			Validate: func(input string) error {
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("enter a valid number")
				}
				return nil
			},
		}
		chainLenStr, err := chainLenPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		chainLength, _ := strconv.Atoi(chainLenStr)

		chainsNumPrompt := promptui.Prompt{
			Label:   "Chains Number",
			Default: fmt.Sprintf("%d", baseChainsNumber),
			Validate: func(input string) error {
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("enter a valid number")
				}
				return nil
			},
		}
		chainsNumStr, err := chainsNumPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		chainsNumber, _ := strconv.Atoi(chainsNumStr)

		configs[i] = TableConfig{
			workerNumber:   baseWorkerNumber,
			chainLength:    chainLength,
			passwordLength: passwordLength,
			chainsNumber:   chainsNumber,
		}
	}

	fmt.Println("\n" + promptui.Styler(promptui.FGBold)("--- Starting Multiple Table Computation ---"))
	for i, cfg := range configs {
		fmt.Printf("Table %d: %d chains, %d password length, %d chain length\n", i+1, cfg.chainsNumber, cfg.passwordLength, cfg.chainLength)
	}

	confirmPrompt := promptui.Select{
		Label: "Proceed with computation?",
		Items: []string{"Yes", "No"},
		Templates: &promptui.SelectTemplates{
			Selected: "{{ . | green }}",
		},
	}
	confirmIndex, _, err := confirmPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}
	if confirmIndex != 0 {
		return
	}

	var wg sync.WaitGroup
	paths := make([]string, numTables)

	for i, cfg := range configs {
		wg.Add(1)
		go func(tableIndex int, config TableConfig) {
			defer wg.Done()
			path := generateTableMultiThread(config.workerNumber, config.chainLength, config.passwordLength, baseCharset, config.chainsNumber, progressBar, strconv.Itoa(tableIndex+1))
			paths[tableIndex] = path
		}(i, cfg)
	}

	wg.Wait()

	fmt.Println("\n" + promptui.Styler(promptui.FGBold, promptui.FGGreen)("All tables generated successfully!"))

	if !tableAutoSort {
		sortPrompt := promptui.Select{
			Label: "Sort all tables?",
			Items: []string{"Yes", "No"},
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
		}
		sortIndex, _, err := sortPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		if sortIndex == 0 {
			fmt.Println("\n" + promptui.Styler(promptui.FGBold)("Sorting all tables..."))
			for i, path := range paths {
				if path != "" {
					fmt.Printf("Sorting Table %d...\n", i+1)
					SortLargeTable(path, sortingChunkSize, progressBar, strconv.Itoa(i+1), baseWorkerNumber)
				}
			}
			if autoRemoveUnsortedTables {
				dir, _ := os.Getwd()
				tablesDir := filepath.Join(dir, "tables")
				removed, err := removeUnsortedTables(tablesDir)
				if err != nil {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Auto remove unsorted tables failed: %v", err)))
				} else if removed > 0 {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Auto removed %d unsorted tables", removed)))
				}
			}
			fmt.Println(promptui.Styler(promptui.FGGreen)("All tables sorted!"))
		}
	} else {
		fmt.Println("\n" + promptui.Styler(promptui.FGBold)("Sorting all tables..."))
		for i, path := range paths {
			if path != "" {
				fmt.Printf("Sorting Table %d...\n", i+1)
				SortLargeTable(path, sortingChunkSize, progressBar, strconv.Itoa(i+1), baseWorkerNumber)
			}
		}
		if autoRemoveUnsortedTables {
			dir, _ := os.Getwd()
			tablesDir := filepath.Join(dir, "tables")
			removed, err := removeUnsortedTables(tablesDir)
			if err != nil {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Auto remove unsorted tables failed: %v", err)))
			} else if removed > 0 {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Auto removed %d unsorted tables", removed)))
			}
		}
		fmt.Println(promptui.Styler(promptui.FGGreen)("All tables sorted!"))
	}
}

func computeMultipleTablesAuto(settings []Setting, progressBar *mpb.Progress) {
	type TableConfig struct {
		workerNumber   int
		chainLength    int
		passwordLength int
		chainsNumber   int
		estimatedSize  string
		actualProb     float64
	}
	var sortingChunkSize int
	var tableAutoSort bool
	var autoRemoveUnsortedTables bool
	var configs []TableConfig
	var totalSizeMB float64
	var baseCharset string
	var baseWorkerNumber int
	configuring := true
	for configuring {
		// Get target probability
		configs = nil
		probPrompt := promptui.Prompt{
			Label:   "Target Success Probability (0.0-1.0)",
			Default: "0.8",
			Validate: func(input string) error {
				prob, err := strconv.ParseFloat(input, 64)
				if err != nil {
					return fmt.Errorf("enter a valid probability between 0.0 and 1.0")
				}
				if prob <= 0 || prob > 1.0 {
					return fmt.Errorf("probability must be between 0.0 and 1.0")
				}
				return nil
			},
		}
		probStr, err := probPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		targetProb, _ := strconv.ParseFloat(probStr, 64)

		// Get password length range
		rangePrompt := promptui.Prompt{
			Label:   "Password Length Range (e.g., 1-6)",
			Default: "1-6",
			Validate: func(input string) error {
				parts := strings.Split(input, "-")
				if len(parts) != 2 {
					return fmt.Errorf("enter range in format 'min-max'")
				}
				min, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				max, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 != nil || err2 != nil {
					return fmt.Errorf("enter valid numbers")
				}
				if min >= max || min < 1 || max > 20 {
					return fmt.Errorf("min must be < max, and both between 1-20")
				}
				return nil
			},
		}
		rangeStr, err := rangePrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		rangeParts := strings.Split(rangeStr, "-")
		minLen, _ := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
		maxLen, _ := strconv.Atoi(strings.TrimSpace(rangeParts[1]))

		// Get base settings
		baseWorkerNumber = getSettingValue(settings, "workerNumber").(int)
		baseChainLength := getSettingValue(settings, "chainLength").(int)
		baseCharset = getSettingValue(settings, "charset").(string)
		sortingChunkSize = getSettingValue(settings, "sortingChunkSize").(int)
		tableAutoSort = getSettingValue(settings, "tableAutoSort").(bool)
		autoRemoveUnsortedTables = getSettingValue(settings, "autoRemoveUnsortedTables").(bool)

		fmt.Println("\n" + promptui.Styler(promptui.FGBold)("--- Auto Configuration Preview ---"))
		fmt.Printf("Target Probability: %.1f%%\n", targetProb*100)
		fmt.Printf("Password Length Range: %d-%d\n", minLen, maxLen)
		fmt.Printf("Chain Length : %d\n\n", baseChainLength)

		for passLen := minLen; passLen <= maxLen; passLen++ {
			requiredChains := calculateRequiredChains(targetProb, baseChainLength, passLen, baseCharset)

			fChains := float64(requiredChains)
			exponent := math.Floor(math.Log10(fChains))
			scale := math.Pow(10, exponent)
			requiredChains = uint64(math.Ceil(fChains/scale) * scale)

			sizeMB, sizeStr := estimateDiskUsage(requiredChains, passLen, baseCharset)
			totalSizeMB += sizeMB

			actualProb := calculateSuccessProbability(int(requiredChains), baseChainLength, passLen, baseCharset)

			config := TableConfig{
				workerNumber:   baseWorkerNumber,
				chainLength:    baseChainLength,
				passwordLength: passLen,
				chainsNumber:   int(requiredChains),
				estimatedSize:  sizeStr,
				actualProb:     actualProb,
			}
			configs = append(configs, config)

			fmt.Printf("Password Length %d: %d chains, %.2f%% success, %s\n",
				passLen, requiredChains, actualProb*100, sizeStr)
		}

		// Show total estimated size
		totalSizeGB := totalSizeMB / 1024
		var totalSizeStr string
		if totalSizeGB >= 1 {
			totalSizeStr = fmt.Sprintf("%.2f GB", totalSizeGB)
		} else {
			totalSizeStr = fmt.Sprintf("%.2f MB", totalSizeMB)
		}
		fmt.Printf("\n%s Total Estimated Size: %s\n", promptui.Styler(promptui.FGYellow)("→"), totalSizeStr)

		// Ask user to adjust chain length if needed
		adjustPrompt := promptui.Select{
			Label: "Adjust chain length for all tables?",
			Items: []string{"Keep current", "Change Settings"},
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
		}
		adjustIndex, _, err := adjustPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		if adjustIndex == 0 {
			configuring = false
		}
	}

	// Start concurrent computation
	numTables := len(configs)
	var wg sync.WaitGroup
	paths := make([]string, numTables)

	fmt.Println("\n" + promptui.Styler(promptui.FGBold)("--- Starting Auto Table Computation ---"))

	for i, cfg := range configs {
		wg.Add(1)
		go func(tableIndex int, config TableConfig) {
			defer wg.Done()
			path := generateTableMultiThread(config.workerNumber, config.chainLength, config.passwordLength, baseCharset, config.chainsNumber, progressBar, strconv.Itoa(i+1))
			paths[tableIndex] = path
		}(i, cfg)
	}

	// Wait for all computations to complete
	wg.Wait()

	fmt.Println("\n" + promptui.Styler(promptui.FGBold, promptui.FGGreen)("All tables generated successfully!"))

	// Ask if user wants to sort the tables
	if !tableAutoSort {
		sortPrompt := promptui.Select{
			Label: "Sort all tables?",
			Items: []string{"Yes", "No"},
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
		}
		sortIndex, _, err := sortPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		if sortIndex == 0 {
			fmt.Println("\n" + promptui.Styler(promptui.FGBold)("Sorting all tables..."))
			for i, path := range paths {
				if path != "" {
					SortLargeTable(path, sortingChunkSize, progressBar, strconv.Itoa(i+1), baseWorkerNumber)
				}
			}
			if autoRemoveUnsortedTables {
				dir, _ := os.Getwd()
				tablesDir := filepath.Join(dir, "tables")
				removed, err := removeUnsortedTables(tablesDir)
				if err != nil {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Auto remove unsorted tables failed: %v", err)))
				} else if removed > 0 {
					fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Auto removed %d unsorted tables", removed)))
				}
			}
			fmt.Println(promptui.Styler(promptui.FGGreen)("All tables sorted!"))
		}
	} else {
		fmt.Println("\n" + promptui.Styler(promptui.FGBold)("Sorting all tables..."))
		for i, path := range paths {
			if path != "" {
				SortLargeTable(path, sortingChunkSize, progressBar, strconv.Itoa(i+1), baseWorkerNumber)
			}
		}
		if autoRemoveUnsortedTables {
			dir, _ := os.Getwd()
			tablesDir := filepath.Join(dir, "tables")
			removed, err := removeUnsortedTables(tablesDir)
			if err != nil {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Auto remove unsorted tables failed: %v", err)))
			} else if removed > 0 {
				fmt.Printf(" %s\n", promptui.Styler(promptui.FGGreen)(fmt.Sprintf("Auto removed %d unsorted tables", removed)))
			}
		}
		fmt.Println(promptui.Styler(promptui.FGGreen)("All tables sorted!"))
	}
}

func settingsMenu(settings []Setting) ([]Setting, error) {
	for {
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

		// Save settings to file
		if err := saveSettings(settings); err != nil {
			fmt.Printf("Warning: Failed to save settings: %v\n", err)
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
					return fmt.Errorf("enter a valid number")
				}
			case "string":
				if len(input) == 0 {
					return fmt.Errorf("input cannot be empty")
				}
			case "bool":
				lower := strings.ToLower(input)
				if lower != "true" && lower != "false" && lower != "yes" && lower != "no" {
					return fmt.Errorf("enter true, false, yes, or no")
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

func searchPasswordKnownLength(targetHash [32]byte, tablePath string, workerNumber int, progressBar *mpb.Progress) (string, bool) {
	header, charset, err := readTableHeader(tablePath)
	if err != nil {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Failed to read table header: %v", err)))
		return "", false
	}

	if header.IsSorted == 0 {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGBold, promptui.FGRed)("THIS TABLE IS NOT SORTED THE SEARCH WON'T WORK"))
	}

	password, found, err := searchTableParallelFromDisk(
		targetHash,
		tablePath,
		workerNumber,
		int(header.ChainLength),
		int(header.PasswordLength),
		charset,
		header.NumChains,
		progressBar,
	)

	if err != nil {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)(fmt.Sprintf("Search failed: %v", err)))
		return "", false
	}

	if found {
		fmt.Printf(" %s %s\n", promptui.Styler(promptui.FGGreen)("Password Found"), password)
	} else {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)("Password Not Found"))
	}
	return password, found
}

func searchPassword(tablesDir string, workerNumber int, tableAutoSelect bool, progressBar *mpb.Progress) {
	validate := func(input string) error {
		match, _ := regexp.MatchString("^[a-fA-F0-9]+$", input)
		if !match {
			return errors.New("invalid format: enter a hex hash")
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
			return errors.New("enter a valid number")
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
			searchPasswordKnownLength([32]byte(hash), selectedTable, workerNumber, progressBar)
		} else {
			fmt.Printf(" %s\n", promptui.Styler(promptui.FGRed)("No Table found for password Length"))
		}
	case 1:
		for i := 1; i < 21; i++ {
			availableTables, _ := selectTable(tablesDir, i, 1, tableAutoSelect)
			if len(availableTables) > 0 {
				fmt.Printf(" %s\n\n", promptui.Styler(promptui.FGBlue)(fmt.Sprintf("Searching for length %d", i)))
				_, found := searchPasswordKnownLength([32]byte(hash), availableTables, workerNumber, progressBar)
				if found {
					break
				}
			}
		}
	}
}

func selectTable(directory string, passwordLength int, sorted int, autoSelect bool) (path string, found bool) {
	tables, _ := listTables(directory, sorted, passwordLength, true)

	if len(tables) == 0 {
		fmt.Printf(" %s\n", promptui.Styler(promptui.FGBold, promptui.FGRed)(fmt.Sprintf("No matching tables found (Sorted Filter: %d, PassLen Filter: %d)", sorted, passwordLength)))
		return "", false
	}

	wDir, _ := os.Getwd()
	if autoSelect {
		path, _ := filepath.Rel(wDir, filepath.Join(directory, tables[0]))
		return path, true
	} else {
		prompt := promptui.Select{
			Label: "Select a Table",
			Items: tables,
			Size:  10,
		}

		_, result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		path, _ := filepath.Rel(wDir, filepath.Join(directory, result))
		return path, true
	}
}
