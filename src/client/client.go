/* 

*/
package client

import (
	"github.com/msgpack/msgpack-rpc/go/rpc"
	"log"
	"net"
	"sync"
	"fmt"
	"time"
)

type Client struct {
	client *rpc.Session
	mutex  *sync.Mutex
	host   string
}

func (c *Client) Send(msg map[string]interface{}) (err error) {
	if c.client == nil {
		return fmt.Errorf("Client connection not made %v", c.host)
	}
	c.mutex.Lock()
	_, err = c.client.Send("send_event", msg)
	c.mutex.Unlock()
	if err != nil {
		go failover(c.host, c)
		return fmt.Errorf("Connection lost %v %v", c.host, err)
	}
	return
}

func NewClient(host string) (c *Client) {
	c = &Client{
		mutex: &sync.Mutex{},
		host:  host,
	}

	conn, err := net.Dial("tcp", host)
	if err != nil {
		log.Printf("ERROR: failed to connect to server. %v", err)
		// TODO failover code
		go failover(host, c)
	} else {
		c.client = rpc.NewSession(conn, false)
	}

	return c
}

func failover(host string, c *Client) {
	connected := false
	for !connected {
		conn, err := net.Dial("tcp", host)
		if err == nil {
			c.client = rpc.NewSession(conn, false)
			log.Printf("Connected to server. %v", host)
			connected = true
		}
		time.Sleep(time.Second)
	}
}
