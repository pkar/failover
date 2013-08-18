package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func main() {
	tmpFile, err := os.OpenFile("failed_events.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Could not open tmp file to rotate to...")
	}
	defer tmpFile.Close()

	pack := map[string]int{}

	for i := 0; i < 1000; i++ {
		pack[fmt.Sprintf("%d", i)] = i
		data, err := json.Marshal(pack)
		if err != nil {
			log.Fatalf("Could not json encode %v", err)
		}
		line := base64.StdEncoding.EncodeToString(data)
		tmpFile.WriteString(fmt.Sprintf("%s\n", line))
	}
}
