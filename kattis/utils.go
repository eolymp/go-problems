package kattis

import "os"

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
