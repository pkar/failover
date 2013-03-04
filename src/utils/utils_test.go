package utils

import (
	"fmt"
	"os"
	"testing"
)

func TestIndexOf(t *testing.T) {
	x := []string{"a", "b", "c", "d"}
	if IndexOf(x, "b") != 1 {
		t.Fatal("Index should be 1")
	}

	if IndexOf(x, "d") != 3 {
		t.Fatal("Index should be 1")
	}
}

func TestNumberOfLines(t *testing.T) {
	file, err := os.OpenFile("test", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.Fatal("Cannot write test file")
	}
	for i := 0; i < 100; i++ {
		file.WriteString(fmt.Sprintln("test"))
	}
	file.Close()
	defer os.Remove("test")

	if NumberOfLines("./test") != 100 {
		t.Fatal("Number of lines should be 100")
	}
}
