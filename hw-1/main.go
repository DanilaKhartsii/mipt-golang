package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Println("Пользователь:", username())
	printArgs(os.Args[1:])
	fmt.Println("Версия Go:", runtime.Version())
}

func username() string {
	for _, key := range []string{"USER", "LOGNAME", "USERNAME"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return "не определено"
}

func printArgs(args []string) {
	if len(args) == 0 {
		fmt.Println("Аргументы: не переданы")
		return
	}

	fmt.Printf("Аргументы (%d):\n", len(args))
	for i, arg := range args {
		fmt.Printf("  %d: %s\n", i+1, arg)
	}
}
