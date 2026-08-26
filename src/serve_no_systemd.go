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

//go:build !systemd

package main

import (
	"log"
	"net"
	"net/http"
)

func serve(c *WebmentionsConfig) error {
	app, server, err := serve_common_setup(c)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", ":8040")
	if err != nil {
		return err
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}

	log.Println("Server HTTP listener closed, waiting for background workers to complete...")
	app.workersWg.Wait()
	log.Println("Server exited cleanly on idle")
	return nil
}
