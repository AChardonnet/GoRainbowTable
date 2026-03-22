package main

import "fmt"

func main() {
	// tui()
	// generateTableMultiThread(20, 5000, 4, generateCharset("a-zA-Z0-9!#@+-"), 5000)
	// SortLargeTable("tables\\2026-03-22_15-48-16.rtable", 1000)
	header, charset, table, _ := loadTableWithHeader("tables\\2026-03-22_15-49-02.rtable")
	// printTable(table)
	fmt.Println(searchTableParallel(hash("papa"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
	fmt.Println(searchTableParallel(hash("vous"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
	fmt.Println(searchTableParallel(hash("date"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
	fmt.Println(searchTableParallel(hash("alex"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
	fmt.Println(searchTableParallel(hash("troy"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
	fmt.Println(searchTableParallel(hash("vers"), table, 20, int(header.ChainLength), int(header.PasswordLength), charset))
}
