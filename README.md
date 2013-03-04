Failover processor
==================

Go file processor that reads log files written in msgpack base64 encoded data.
Use case is for instance failed request is written to log file for example when service is down and then resent when service is back up.

Worker
======

Build:

    cd go
    GOPATH=$(pwd)
    ./script/build.sh

Usage examples:

    go run failover.go

Or:

    ./failover_linux -env=production -applog=/var/log/app.log -errlog=$(pwd)/log/failover.log

Options: 

    -applog="": path to application logging(default STDOUT)
    -debug=false: Send actual event or just mock if enabled
    -env="development": development:testing:staging:sandbox:production
    -errlog="failover.log": path to error logged files
    -maxbytes=1048576: Maximum number of bytes before rotating
    -maxprocessing=4: Maximum number of workers on rotated files
    -syslog=false: log to syslog

Testing:

    cd go
    GOPATH=$(pwd)
    go test failover
    go test client
    go test utils 

Description
===========

Files get rotated after set time or size configured in a
const variable in src/failover/failover.go

Files are locked to sync on rotating using flock.
Periodicaly if there are no timestamped files to work on
the err.log file will be moved to a worker.

                  W1
                  |
    err.log >> err.log.1234


Rotated files are err.log.{timestamp}, that way when one 
worker process removes a file, a new addition is added to the 
end and sorting is easier.


      |          W1                W2
                 |                 |
    err.log  err.log.1234      err.log.1235      err.log.1236

As files are processed, each line is added to filename{.tmp} so that
on restart progress is remembered.

Some processing and W2 finishes file...


      |          W1                W2
                 |                 |
    err.log  err.log.1234      err.log.1236      err.log.1239

W2 moves to the next timestamped file.

As rotated files get added, the disk space is also considered(TODO) and
the oldest(NOTE up for discussion) files gets removed.

