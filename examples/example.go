package main

import (
	"encoding/json"
	"failover"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {

	var pr failover.Processor
	pr = func(payload interface{}) error {
		log.Println("Re-sending payload ", payload)
		return nil
	}

	flr, err := failover.NewFailover(pr)
	if err != nil {
		log.Fatal(err)
	}
	go flr.FileWatcher()
	go flr.FileRotator()

	payload := map[string]interface{}{"test": "1"}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post("http://localhost:123456/", "application/json", strings.NewReader(string(b)))
	if err != nil {
		flr.Write(string(b))
		log.Println(err)
	}
	log.Println(resp, " Resp should be <nil> write to failed_events.log, the failover processor should rotate the file and retry")
	time.Sleep(5 * time.Second)
}
