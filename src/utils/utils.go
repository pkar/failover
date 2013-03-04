/* 

*/
package utils

import (
	"os/exec"
	"reflect"
	"strconv"
	"strings"
)

// Test if interface has item
func IndexOf(slice interface{}, val interface{}) int {
	sv := reflect.ValueOf(slice)

	for i := 0; i < sv.Len(); i++ {
		if sv.Index(i).Interface() == val {
			return i
		}
	}
	return -1
}

// Get the number of lines for a path.
// NOTE requires wc
func NumberOfLines(path string) int {
	out, err := exec.Command("wc", "-l", path).Output()
	if err != nil {
		out = []byte("1")
	}
	wcOut := strings.SplitN(strings.Trim(string(out), " "), " ", 2)
	startLine, err := strconv.Atoi(wcOut[0])
	if err != nil {
		startLine = 1
	}
	return startLine
}
