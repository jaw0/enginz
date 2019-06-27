// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Dec-11 15:40 (EST)
// Function: because net/http:Server.ErrorLog is not an interface

package enginz

import (
	"fmt"
	"log"
	"os"
	"time"
)

type engWrite struct {
	s *Server
}

func (s *Server) newErrorLogger() *log.Logger {

	if s.ErrorLog == "" {
		return nil
	}

	s.errch = make(chan string, logQueueSize)
	l := log.New(engWrite{s}, "", 0)

	s.done.Add(1)
	go s.errlogger()

	return l
}

func (s *Server) Error(b []byte) (int, error) {

	if s.Log != nil {
		s.Log.Verbose("%s", b)
	}

	if s.errch != nil {
		s.errch <- string(b)
	}

	return len(b), nil
}

func (e engWrite) Write(b []byte) (int, error) {

	return e.s.Error(b)
}

func (s *Server) errlogger() {

	for {
		select {
		case msg, ok := <-s.errch:
			if !ok {
				return
			}

			// it is expected that errors will be few + far between.
			// performance is not an issue
			if s.ErrorLog != "" {
				w, err := os.OpenFile(s.ErrorLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE /*sic*/, 0666)
				if err != nil {
					s.Report.Problem("cannot open log file '%s': %v", s.ErrorLog, err)
				} else {
					fmt.Fprintf(w, "%s %s", time.Now().Format("2006-01-02T15:04:05"), msg)
					w.Close()
				}
			}
		}
	}
}
