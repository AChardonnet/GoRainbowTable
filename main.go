package main

const (
	charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
	passwordLength = 6
	chainLength    = 1000
	chainsNumber   = 100000
	workerNumber   = 8
)

func main() {
	// generateTableMultiThread(true)
	table, _ := loadTable("2026-03-22_01-15-38.rtable")
	printTable(table)
}
