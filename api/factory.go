package api

import (
	"net/http"
	"time"
)

func NewClient(token, server string) Client {
	transport := &http.Transport{
		MaxIdleConns:    5,
		IdleConnTimeout: 30 * time.Second,
	}

	return &client{
		token:      token,
		server:     server,
		cmdInbound: make(chan []byte),
		reConn:     make(chan struct{}),
		pending:    make(chan *message, 100),
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
	}
}
