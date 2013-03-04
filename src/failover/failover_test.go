package failover

import (
	"client"
	"testing"
)

func TestFailover(t *testing.T) {
	quit := make(chan struct{})
	done := make(chan string)

	clients := make([]*client.Client, 0, 0)

	fa := &Failover{
		Debug:    true,
		Env:      "staging",
		MaxBytes: 100,
		Errlog:   "test",
		Clients:  clients,
		Quit:     quit,
		Done:     done,
	}
	fa.Close()
}
