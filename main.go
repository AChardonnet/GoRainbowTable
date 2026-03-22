package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
)

func main() {
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
