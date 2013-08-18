Failover processor
==================

Go file processor that reads log files written in json and base64 encoded data.
Use case is for instance failed send request for example when a service is down, which is then resent when service is back up.

Example
=======

    go run examples/example.go

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

