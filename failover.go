package failover

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	watchTicker      = 1                   // seconds look for new rotated files to process
	rotateTickerSize = 10                  // seconds rotate on file size change
	rotateTickerTime = 1                   // seconds rotate on this interval if there are no jobs
	maxProcessing    = 4                   // number of workers processing events
	maxBytes         = 1048576             // number of bytes before rotation
	failoverFilePath = "failed_events.log" // name of failover file
)

// Processor is a function defined elsewhere that
// handles the unpacked data.
type Processor func(interface{}) error

// Failover [...]
type Failover struct {
	MaxProcessing int           // maximum number of workers reading rotated files
	MaxBytes      int64         // Maximum bytes before rotation
	Errlog        string        // Path to main error log file
	Processor     Processor    // A function that does things with the dumped data
	Quit          chan struct{} // Shutdown signal for go routines
	Done          chan string   // file processor
	File          *os.File
}

// NewFailover maps a processing function to newly found events.
func NewFailover(pr Processor) (*Failover, error) {
	f, err := os.OpenFile(failoverFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	quit := make(chan struct{})
	done := make(chan string)

	failover := &Failover{
		MaxProcessing: maxProcessing,
		MaxBytes:      maxBytes,
		Errlog:        failoverFilePath,
		File:          f,
		Quit:          quit,
		Done:          done,
		Processor:     pr,
	}

	return failover, nil
}

// Write writes a json marshalled, base64 encoded string
// to file. `data` must be json serializable.
func (f *Failover) Write(data interface{}) {
	msg, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		return
	}
	encodedData := string(base64.StdEncoding.EncodeToString(msg))
	fd := int(f.File.Fd())
	syscall.Flock(fd, syscall.LOCK_SH)
	f.File.WriteString(fmt.Sprintf("%s\n", encodedData))
	syscall.Flock(fd, syscall.LOCK_UN)
}

// Read decodes json and base 64 into an interface
func (f *Failover) Read(encoded string) (interface{}, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded[:len(encoded)-1])
	if err != nil {
		return nil, err
	}
	var unpacked interface{}
	err = json.Unmarshal(decoded, &unpacked)
	if err != nil {
		return nil, err
	}
	return unpacked, nil
}

// IndexOf tests if interface has item.
func IndexOf(slice interface{}, val interface{}) int {
	sv := reflect.ValueOf(slice)

	for i := 0; i < sv.Len(); i++ {
		if sv.Index(i).Interface() == val {
			return i
		}
	}
	return -1
}

// NumberOfLines gets the number of lines for a path. Requires wc.
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

// fileProcessor [...]
func (f *Failover) fileProcessor(path string) (err error) {
	log.Printf("Processing file...%v", path)

	tPath := fmt.Sprintf("%v.tmp", path)
	tmpFile, err := os.OpenFile(tPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println("ERROR: Cannot open tmp file ", tPath)
		return
	}
	defer tmpFile.Close()

	file, err := os.Open(path)
	if err != nil {
		log.Printf("ERROR: Cannot open file %v", path)
		return
	}
	defer file.Close()

	buff := bufio.NewReader(file)

	// Advance line pointer to where n lines tmp file left off at
	startLine := NumberOfLines(tPath)
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
			msg, parseErr := f.Read(line)
			if parseErr != nil {
				log.Printf("ERROR: line parsing error: %v", line)
				moveOn = true
				continue
			}
			log.Println("Retrying message:", msg)

			sendErr := f.Processor(msg)
			if sendErr == nil {
				moveOn = true
				continue
			}
			// Retry in a second
			log.Printf("%v", sendErr)
			time.Sleep(time.Second)
		}
		tmpFile.WriteString(fmt.Sprintf("%s", string(line)))
		line, err = buff.ReadString('\n')
	}

	log.Printf("Done processing file...%v", path)
	return nil
}

// worker [...]
func (f *Failover) worker(id int, jobs <-chan string, results chan<- string) {
	for path := range jobs {
		log.Printf("worker %d processing job %s", id, path)
		err := f.fileProcessor(path)
		if err == nil {
			results <- path
		}
	}
}

// FileWatcher checks for new files to process after they've been rotated
func (f *Failover) FileWatcher() {
	log.Println("Starting file watcher")
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
					if !strings.Contains(path, ".tmp") && !(IndexOf(processQueue, path) != -1) {
						mutex.Lock()
						processQueue = append(processQueue, path)
						jobs <- path
						mutex.Unlock()
					}
				}
			}
		case path := <-results:
			index := IndexOf(processQueue, path)
			if index != -1 {
				mutex.Lock()
				os.Remove(path)
				os.Remove(path + ".tmp")
				log.Printf("Removed %s %s", path, path+".tmp")
				processQueue = append(processQueue[:index], processQueue[index+1:]...)
				mutex.Unlock()
			}
		case <-f.Quit:
			quitting = true
			break
		}
	}
}

// rotateFile [...]
func (f *Failover) rotateFile() {
	log.Printf("Rotating file...%v", f.Errlog)

	tPath := fmt.Sprintf("%v.%v", f.Errlog, fmt.Sprintf("%d", time.Now().Unix()))
	tmpFile, err := os.OpenFile(tPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("Could not open tmp file to rotate to...%v", tPath)
		return
	}
	defer tmpFile.Close()

	file, err := os.OpenFile(f.Errlog, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Printf("Could not open main log file to rotate from...%v", f.Errlog)
		return
	}
	defer file.Close()

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
		log.Printf("Truncating %v %v bytes", f.Errlog, stat.Size())
		file.Truncate(0)
	}

	log.Printf("Done rotating file...%v", f.Errlog)
}

// FileRotator every so often check file size and rotate.
func (f *Failover) FileRotator() {
	log.Println("Starting file rotator")
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

// Close [...]
func (f *Failover) Close() {
	f.File.Close()
	close(f.Quit)
}
