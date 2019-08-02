// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Dec-11 15:40 (EST)
// Function: simple web server framework

// enginz is a small go web framework.
package enginz

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

type HandlerFunc http.HandlerFunc

// Service defines the Service Endpoint for web serving
type Service struct {
	// Addr - the address to serve on. eg. ":80"
	Addr string
	// TLSConfig - a tls.Config for serving TLS.
	TLSConfig *tls.Config
	// TLSKey, TLSCert - files containing TLS key + cert
	TLSKey  string
	TLSCert string
	www     http.Server
}

// Reporter interface for reporting serious problems.
// production services may wish to wire these to email a stack trace to the webmaster
type Reporter interface {
	Problem(string, ...interface{})
	Fatal(string, ...interface{})
}

type Logger interface {
	Debug(string, ...interface{})
	Verbose(string, ...interface{})
	Problem(string, ...interface{})
}

type report struct{}

// Server defines the web server configuration.
//
// enginz.Server{
//   Service: {
//     { Addr: ":80" },
//     { Addr: ":443", TLSCert: "sample.crt", TLSKey: "sample.key" },
//   },
//   AccessLog: "/var/log/access.log",
//   Handler:   router,
// }
//
type Server struct {
	// Service defines all of the Service Endpoints. feel free to mix-and-match http + https
	Service []Service
	// AccessLog specifies the file to use for logging. leave empty for none.
	AccessLog string
	ErrorLog  string
	Log       Logger
	// Handler specifies a standard http.Handler. required
	// defaults to http.DefaultServeMux
	Handler  http.Handler
	TraceID  string      // TraceID specifies a value for the X-Origin-ID header
	ServerID string      // ServerID specifies a value for the Server header
	Error500 HandlerFunc // Error500 specifies a http.HandlerFunc for generating 500 server errors
	Report   Reporter    // Report specifies an error Reporter
	Collect  Collector   // Collect specifies a stats Collector
	logch    chan *Collect
	errch    chan string
	done     sync.WaitGroup
}

// Collect provides data to the statistics Collector
type Collect struct {
	Req    *http.Request
	Size   int64
	Status int
	Usec   int
}
type Collector func(*Collect)

type responseWriter struct {
	w      http.ResponseWriter
	size   int64
	status int
}

const logQueueSize = 1000

// Serve starts the web server.
//
// s := &enginz.Server{ ... }
// either:
//   s.Serve()
// or
//   enginz.Serve(s)
//
func Serve(s *Server) {
	s.Serve()
}

func (s *Server) Serve() {

	if s.Report == nil {
		s.Report = report{}
	}

	if s.ServerID == "" {
		s.ServerID = "enginZ!"
	}

	if s.Handler == nil {
		s.Handler = http.DefaultServeMux
	}

	s.newAccessLogger()
	errz := s.newErrorLogger()

	for i, _ := range s.Service {
		ss := &s.Service[i]

		www := http.Server{
			Addr:      ss.Addr,
			Handler:   s,
			TLSConfig: ss.TLSConfig,
			ErrorLog:  errz, // why is this not simply an interface?
		}

		ss.www = www

		s.done.Add(1)

		go func() {
			defer s.done.Done()
			// either a tls.Config or a Key+Cert pair (or both)
			if ss.TLSConfig != nil || (ss.TLSKey != "" && ss.TLSCert != "") {
				www.ListenAndServeTLS(ss.TLSCert, ss.TLSKey)
			} else {
				www.ListenAndServe()
			}
		}()
	}

	s.done.Wait()
}

// Shutdown stops the server.
// see also: net/http Shutdown()
func (s *Server) Shutdown(ctx context.Context) {

	var wg sync.WaitGroup

	close(s.logch)
	close(s.errch)

	for _, ss := range s.Service {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ss.www.Shutdown(ctx)
		}()
	}

	wg.Wait()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	defer func() {
		// because bugs
		if r := recover(); r != nil {
			s.Report.Problem("PANIC! %s -> %s\n%s\n", req.RemoteAddr, req.RequestURI, string(debug.Stack()))
			s.serverError(w, req)
		}
	}()

	// set headers
	header := w.Header()
	if s.ServerID != "" {
		header.Set("Server", s.ServerID)
	}
	if s.TraceID != "" {
		header.Set("X-Origin-Id", s.TraceID)
	}

	lw := &responseWriter{w: w}

	// run the provided handler, and time it
	t0 := time.Now()
	s.Handler.ServeHTTP(lw, req)
	t1 := time.Now()
	dt := t1.Sub(t0)

	if lw.status == 0 {
		lw.status = 200
	}

	// collect our stats
	c := &Collect{
		Usec:   int(dt.Nanoseconds() / 1000),
		Size:   lw.size,
		Status: lw.status,
		Req:    req,
	}

	// log it
	s.log(c)

	if s.Collect != nil {
		s.Collect(c)
	}
}

func (s *Server) serverError(w http.ResponseWriter, req *http.Request) {

	defer func() {
		if r := recover(); r != nil {
			// because bugs everywhere!
			s.Report.Problem("PANIC in PANIC handler! %s -> %s", req.RemoteAddr, req.RequestURI)
			w.WriteHeader(500)
		}
	}()

	if s.Error500 != nil {
		s.Error500(w, req)
		return
	}

	w.WriteHeader(500)
	fmt.Fprintf(w, "Something has gone horribly, horribly, wrong.\n")

}

// ################################################################

var blankGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x2c,
	0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02,
	0x02, 0x44, 0x01, 0x00, 0x3b,
}

// BlankGif serves out a blank 1x1 pixel gif.
func BlankGif(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "image/gif")
	w.Write(blankGIF)
}

// ################################################################

// default Reporter interface

func (report) Problem(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, s, args...)
}

func (report) Fatal(s string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, s, args...)
	os.Exit(-1)
}
