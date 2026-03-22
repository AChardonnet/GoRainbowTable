package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
)

func tui() {
	stay := true

	//default Settings
	var (
		charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
		passwordLength = 6
		chainLength    = 5000
		chainsNumber   = 10000000
		workerNumber   = 20
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
			computeTable(workerNumber, chainLength, passwordLength, charset, chainsNumber)
		case 3:
			settingsMenu(workerNumber, chainLength, passwordLength, charset, chainsNumber)
		case 4:
			stay = false
		}
	}
}

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

func computeTable(workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int) {
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
		generateTableMultiThread(workerNumber, chainLength, passwordLength, charset, chainsNumber)
		return
	case 1:
		err, workerNumber, chainLength, passwordLength, charset, chainsNumber := settingsMenu(workerNumber, chainLength, passwordLength, charset, chainsNumber)
		if err != nil {
			log.Fatal(err)
		}
		computeTable(workerNumber, chainLength, passwordLength, charset, chainsNumber)
	case 2:
		return
	}

}

func settingsMenu(workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int) (error, int, int, int, string, int) {
	for {
		// 1. Define the labels with current values for the menu
		options := []string{
			fmt.Sprintf("Workers: %d", workerNumber),
			fmt.Sprintf("Chain Length: %d", chainLength),
			fmt.Sprintf("Password Length: %d", passwordLength),
			fmt.Sprintf("Chains Number: %d", chainsNumber),
			fmt.Sprintf("Charset: %s", charset),
			"Back to Main Menu",
		}

		menu := promptui.Select{
			Label: "Select a setting to modify",
			Items: options,
			Templates: &promptui.SelectTemplates{
				Selected: "{{ . | green }}",
			},
			Size: 6,
		}

		index, _, err := menu.Run()
		if err != nil {
			return err, 0, 0, 0, "", 0
		}

		// Handle "Back" option
		if index == 5 {
			break
		}

		// 2. Prompt for the new value
		err, workerNumber, chainLength, passwordLength, charset, chainsNumber = updateSetting(
			index, workerNumber, chainLength, passwordLength, charset, chainsNumber,
		)
	}
	return nil, workerNumber, chainLength, passwordLength, charset, chainsNumber
}

func updateSetting(index int, workerNumber int, chainLength int, passwordLength int, charset string, chainsNumber int) (error, int, int, int, string, int) {
	labels := []string{"Workers", "Chain Length", "Password Length", "Chains Number", "Charset"}

	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Enter new value for %s", labels[index]),
		// Basic validation to ensure numbers are numbers
		Validate: func(input string) error {
			if index < 4 { // Indices 0-3 are integers
				_, err := strconv.Atoi(input)
				if err != nil {
					return fmt.Errorf("please enter a valid number")
				}
			}
			if len(input) == 0 {
				return fmt.Errorf("input cannot be empty")
			}
			return nil
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return err, 0, 0, 0, "", 0
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
	}
	return nil, workerNumber, chainLength, passwordLength, charset, chainsNumber
}
