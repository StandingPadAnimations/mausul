// Copyright (C) 2026 Maryam Stellamaris <maryam@standingpad.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build systemd

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/coreos/go-systemd/v22/activation"
)

func serve(c *WebmentionsConfig) error {
	app, server, err := serve_common_setup(c)
	if err != nil {
		return err
	}
	listeners, err := activation.Listeners()
	if err != nil {
		return err
	}
	if len(listeners) == 0 {
		return fmt.Errorf("No socket activation fds found")
	}
	log.Println("Server started on socket activation fd")
	var wg sync.WaitGroup
	for _, l := range listeners {
		defer l.Close()
		wg.Add(1)
		go func(listener net.Listener) {
			defer wg.Done()
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Printf("server failed: %v", err)
			}
		}(l)
	}
	wg.Wait()

	log.Println("Server HTTP listener closed, waiting for background workers to complete...")
	app.workersWg.Wait()
	log.Println("Server exited cleanly on idle")
	return nil
}
