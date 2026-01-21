package main

import "os"

func checkProc(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	isRegular := stat.Mode()&os.ModeType == 0

	return isRegular
}
