package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
)

func main() {
	stay := true

	dir, _ := os.Getwd()
	tablesDir := filepath.Join(dir, "tables")
	for stay {
		items := []string{"List Tables", "Compute Table", "Search Table", "Exit"}

		prompt := promptui.Select{
			Label: "Select Action",
			Items: items,
			// You can customize the icons too
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
		case 3:
			stay = false
		}
	}
}
