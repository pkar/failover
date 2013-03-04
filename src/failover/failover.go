/*

*/
package failover

import (
	"bufio"
	"client"
	"encoding/base64"
	"fmt"
	"github.com/ugorji/go-msgpack"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"utils"
)

const (
	watchTicker      = 3  // seconds look for new rotated files to process
	rotateTickerSize = 2  // seconds rotate on file size change
	rotateTickerTime = 30 // seconds rotate on this interval if there are no jobs
)

type Failover struct {
	MaxProcessing int // maximum number of workers reading rotated files
	Debug         bool
	Clients       []*client.Client
	Env           string
	MaxBytes      int64         // Maximum bytes before rotation
	Errlog        string        // Path to main error log file
	Quit          chan struct{} // Shutdown signal for go routines
	Done          chan string   // file processor
}

func (f *Failover) client() *client.Client {
	var c *client.Client
	numClients := len(f.Clients)
	switch numClients {
	case 0:
		c = nil
	case 1:
		c = f.Clients[0]
	default:
		// Random client
		c = f.Clients[rand.Intn(numClients)]
	}
	return c
}

// returns sucess, senderror, parseerror
func (f *Failover) sendEvent(encoded string) (bool, error, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered %v\n", r)
		}
	}()

	decoded, err := base64.StdEncoding.DecodeString(encoded[:len(encoded)-1])
	if err != nil {
		return false, nil, err
	}

	var unpacked map[string]interface{}
	err = msgpack.Unmarshal(decoded, &unpacked, nil)
	if err != nil {
		return false, nil, err
	}

	if !f.Debug {
		c := f.client()
		if c == nil {
			return false, fmt.Errorf("ERROR: no clients defined. %v"), nil
		}
		err := c.Send(unpacked)
		if err != nil {
			if !strings.Contains(fmt.Sprintf("%v", err), "Invalid message format") {
				return false, fmt.Errorf("ERROR: failed to send %v. %v", unpacked, err), nil
			}
		}
	}
	log.Printf("Sent message %v\n", unpacked)
	return true, nil, nil
}

func (f *Failover) fileProcessor(path string) (err error) {
	log.Printf("Processing file...%v\n", path)

	tPath := fmt.Sprintf("%v.tmp", path)
	tmpFile, err := os.OpenFile(tPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("ERROR: Cannot open tmp file %v\n", tPath)
		return
	}
	defer tmpFile.Close()

	file, err := os.Open(path)
	if err != nil {
		log.Printf("ERROR: Cannot open file %v\n", path)
		return
	}
	defer file.Close()

	buff := bufio.NewReader(file)

	// Advance line pointer to where n lines tmp file left off at
	startLine := utils.NumberOfLines(tPath)
	line, err := buff.ReadString('\n')
	for i := 1; i <= startLine; i++ {
		line, err = buff.ReadString('\n')
		if err != nil {
			break
		}
	}

	// Iterate each line in file
	for err == nil {
		moveOn := false
		for !moveOn {
			// Retry send events until done
			// parse errors just move on
			// send errors are retried every second
			ok, sendErr, parseErr := f.sendEvent(line)
			if ok {
				moveOn = true
			}
			if parseErr != nil {
				log.Printf("ERROR: line parsing error: %v\n", line)
				moveOn = true
			}
			if sendErr != nil {
				log.Printf("%v\n", sendErr)
				time.Sleep(time.Second)
			}
		}
		tmpFile.WriteString(fmt.Sprintf("%s", string(line)))
		line, err = buff.ReadString('\n')
	}

	log.Printf("Done processing file...%v\n", path)
	return nil
}

func (f *Failover) worker(id int, jobs <-chan string, results chan<- string) {
	for path := range jobs {
		log.Println("worker", id, "processing job", path)
		err := f.fileProcessor(path)
		if err == nil {
			results <- path
		}
	}
}

// Check for new files to process after they've been rotated
func (f *Failover) FileWatcher() {
	ticker := time.NewTicker(watchTicker * time.Second)
	quitting := false

	jobs := make(chan string, 100)
	results := make(chan string, 100)

	// Startup the workers
	for w := 0; w < f.MaxProcessing; w++ {
		go f.worker(w, jobs, results)
	}

	// Currently being processed files
	processQueue := make([]string, 0, 10)
	mutex := &sync.Mutex{}

	for !quitting {
		select {
		case <-ticker.C:
			paths, err := filepath.Glob(f.Errlog + ".*")
			if err == nil {
				for _, path := range paths {
					if !strings.Contains(path, ".tmp") && !(utils.IndexOf(processQueue, path) != -1) {
						mutex.Lock()
						processQueue = append(processQueue, path)
						jobs <- path
						mutex.Unlock()
					}
				}
			}
		case path := <-results:
			index := utils.IndexOf(processQueue, path)
			if index != -1 {
				mutex.Lock()
				os.Remove(path)
				os.Remove(path + ".tmp")
				log.Println("Removed", path, path+".tmp")
				processQueue = append(processQueue[:index], processQueue[index+1:]...)
				mutex.Unlock()
			}
		case <-f.Quit:
			quitting = true
			break
		}
	}
}

func (f *Failover) rotateFile() {
	log.Printf("Rotating file...%v\n", f.Errlog)

	tPath := fmt.Sprintf("%v.%v", f.Errlog, fmt.Sprintf("%d", time.Now().Unix()))
	tmpFile, err := os.OpenFile(tPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Could not open tmp file to rotate to...%v", tPath)
		return
	}
	file, err := os.OpenFile(f.Errlog, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Could not open main log file to rotate from...%v", f.Errlog)
		return
	}

	defer file.Close()
	defer tmpFile.Close()

	syscall.Flock(int(file.Fd()), syscall.LOCK_SH)
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_SH)
	defer syscall.Flock(int(tmpFile.Fd()), syscall.LOCK_UN)

	buff := bufio.NewReader(file)
	line, err := buff.ReadString('\n')
	for err == nil {
		tmpFile.WriteString(fmt.Sprintf("%s", line))
		line, err = buff.ReadString('\n')
	}

	stat, err := file.Stat()
	if err == nil {
		log.Printf("Truncating %v %v bytes\n", f.Errlog, stat.Size())
		file.Truncate(0)
	}

	log.Printf("Done rotating file...%v\n", f.Errlog)
}

// Every so often check file size
// and rotate.
func (f *Failover) FileRotator() {
	tickerSize := time.NewTicker(rotateTickerSize * time.Second)
	tickerTime := time.NewTicker(rotateTickerTime * time.Second)
	quitting := false
	for !quitting {
		select {
		case <-tickerSize.C:
			// TODO refactor duplicate code
			file, err := os.Open(f.Errlog)
			if err == nil {
				stat, err := file.Stat()
				if err == nil {
					if stat.Size() > f.MaxBytes {
						file.Close()
						f.rotateFile()
					}
				}
			}
		case <-tickerTime.C:
			// TODO refactor duplicate code
			file, err := os.Open(f.Errlog)
			if err == nil {
				stat, err := file.Stat()
				if err == nil {
					if stat.Size() > 0 {
						file.Close()
						f.rotateFile()
					}
				}
			}
		case <-f.Quit:
			quitting = true
			break
		}
	}
}

func (f *Failover) Close() {
	close(f.Quit)
}
