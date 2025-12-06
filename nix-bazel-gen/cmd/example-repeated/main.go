package main

import (
	"flag"
	"fmt"
	"strings"
)

// repeatedStringFlag is a custom type that implements flag.Value
// to handle repeated command-line arguments.
type repeatedStringFlag []string

// String is the method to format the flag's value, part of the flag.Value interface.
// The flag package may call this when printing usage.
func (i *repeatedStringFlag) String() string {
	return strings.Join(*i, ", ")
}

// Set is the method to set the flag value, part of the flag.Value interface.
// Set's argument is a string to be parsed to set the flag.
// It's a comma-separated list, so we split it.
func (i *repeatedStringFlag) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// Define variables for the custom type.
	var keys repeatedStringFlag
	var values repeatedStringFlag

	// Bind the flags to the variables.
	flag.Var(&keys, "key", "A key flag that can be repeated")
	flag.Var(&values, "value", "A value flag that can be repeated")

	flag.Parse()

	if len(keys) != len(values) {
		fmt.Printf("Error: number of keys (%d) does not match number of values (%d)\n", len(keys), len(values))
		return
	}

	// Create the map
	myMap := make(map[string]string)
	for i := 0; i < len(keys); i++ {
		myMap[keys[i]] = values[i]
	}

	fmt.Println("Constructed Map:")
	for k, v := range myMap {
		fmt.Printf("  %s -> %s\n", k, v)
	}
}
