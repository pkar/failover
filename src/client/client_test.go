package client

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test")
	if c.host != "test" {
		t.Fatal("")
	}
}

func TestSend(t *testing.T) {
	c := NewClient("test")
	err := c.Send(map[string]interface{}{"a": "b"})
	if err == nil {
		t.Fatal("Should get connection error")
	}
}
