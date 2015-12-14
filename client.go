// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"net/url"
	"os"
	"os/signal"
	"time"
)

type CommandSingle struct {
	Command string `json:"command"`
}

type CommandMulti struct {
	Command []string `json:"command"`
}

type Wall struct {
	Alive    bool     `json:"alive"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

type Player struct {
	Alive    bool     `json:"alive"`
	Name     string   `json:"name"`
	Id       string   `json:"id"`
	Score    int      `json:"score"`
	Position Position `json:"position"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type World struct {
	Walls   []Wall `json:"walls"`
	Players []Wall `json:"players"`
}

type BomberClient struct {
	OutMessages chan []byte
	URL         string
	Name        string
}

func NewBomberClient() (bc *BomberClient) {
	bc = &BomberClient{}
	bc.OutMessages = make(chan []byte)
	return bc
}

func (bc *BomberClient) Command(commands []string) {

	var command interface{}

	if len(commands) > 1 {
		command = &CommandMulti{
			Command: commands,
		}
	} else {
		command = &CommandSingle{
			Command: commands[0],
		}
	}

	data, _ := json.Marshal(
		command,
	)

	bc.OutMessages <- data

}

func (bc *BomberClient) Look() {
	bc.Command([]string{"Look"})
}

func (bc *BomberClient) SetName(name string) {
	bc.Command([]string{"SetName", bc.Name})
}

func (bc *BomberClient) Init() {

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: bc.URL, Path: "/"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			go bc.HandleMessage(message)
		}
	}()

	// set name
	go bc.SetName(bc.Name)

	// add a ticker
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		// ticker for get the new state of the world
		case t := <-ticker.C:
			bc.Look()
			log.Warnf(t.String())
		// sent out messages
		case msg := <-bc.OutMessages:
			err := c.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Warn("write error:", err)
			}
			log.Infof("sent: %s", msg)
		// close connection after interrupt
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}

}

func (bc *BomberClient) HandleMessage(message []byte) {
	status := &World{}
	err := json.Unmarshal(message, status)
	if err != nil {
		log.Warn("Error parsing message", err)
	}
	log.Debugf("Status: %+v", status)
}

func main() {
	log.SetLevel(log.DebugLevel)

	bc := NewBomberClient()
	bc.URL = "10.112.155.244:8080"
	bc.Name = "Golang spectator"
	bc.Init()
}
