// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// native-snapshot-guest is a PID 1 fixture for the real-VMM snapshot test.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/mdlayher/vsock"
	"golang.org/x/sys/unix"
)

var state struct {
	sync.Mutex
	counter, connections, active uint64
}

func main() {
	if err := unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, ""); err != nil {
		log.Fatal(err)
	}
	if err := unix.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		log.Fatal(err)
	}
	listener, err := vsock.Listen(1234, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("native snapshot guest ready")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		state.Lock()
		state.connections++
		id := state.connections
		state.active++
		state.Unlock()
		go serve(conn, id)
	}
}

func serve(conn net.Conn, id uint64) {
	defer conn.Close()
	defer func() {
		state.Lock()
		state.active--
		state.Unlock()
	}()
	decoder, encoder := json.NewDecoder(conn), json.NewEncoder(conn)
	for {
		var request struct {
			Op    string
			Value uint64
		}
		if err := decoder.Decode(&request); err != nil {
			return
		}
		state.Lock()
		switch request.Op {
		case "set":
			state.counter = request.Value
		case "inc":
			state.counter++
		}
		reply := struct{ Counter, Connection, Active uint64 }{state.counter, id, state.active}
		state.Unlock()
		if err := encoder.Encode(reply); err != nil {
			return
		}
	}
}
