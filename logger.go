// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Dec-11 15:40 (EST)
// Function: access log

package enginz

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func (s *Server) newAccessLogger() {

	if s.AccessLog == "" {
		return
	}

	s.logch = make(chan *Collect, logQueueSize)
	s.done.Add(1)
	go s.logger()
}

// RotateLog closes and reopens the log file
func (s *Server) RotateLog() {

	if s.logch == nil {
		return
	}

	// send empty msg as signal
	s.logch <- &Collect{}
}

func (s *Server) log(c *Collect) {

	if s.Log != nil {
		// user provided interface
		verboseLog(s.Log, c)
	}

	if s.logch == nil {
		return
	}

	// if the logger cannot keep up, drop the log, don't block
	select {
	case s.logch <- c:
	default:
		break
	}
}

func (s *Server) logger() {

	defer s.done.Done()

	// open file
	w, err := os.OpenFile(s.AccessLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE /*sic*/, 0666)

	if err != nil {
		s.Report.Fatal("cannot open log file '%s': %v", s.AccessLog, err)
		return
	}

	defer func() {
		w.Close()
	}()

	for {
		select {
		case msg, ok := <-s.logch:
			if !ok {
				return
			}
			if msg.Req == nil {
				// rotate log
				wx, err := os.OpenFile(s.AccessLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE /*sic*/, 0666)
				if err != nil {
					s.Report.Problem("cannot open log file '%s': %v", s.AccessLog, err)
				} else {
					w.Close()
					w = wx
				}
				continue
			}

			writeLog(w, msg)
		}
	}
}

// AC log format. similar to:
//  apache: '$remote_addr - $msec $http_host $status $body_bytes_sent $request_time "$request" "$http_referer" "$http_user_agent"';
//  nginx:  "%h %c %{%Y-%m-%dT%H:%M:%S}t %v %>s %b %P %T \"%r\" \"%{Referer}i\" \"%{User-Agent}i\"" combined
//
// no, you cannot change the format, but it is simple to parse. break out your perl/sed/awk
// all fields which can contain whitespace are percent-encoded, so you can simply split on space (how cool is that!)

func writeLog(w io.Writer, msg *Collect) {

	req := msg.Req
	header := req.Header
	rfr := header.Get("Referer")
	ua := header.Get("User-Agent")

	if rfr == "" {
		rfr = "-"
	}
	if ua == "" {
		ua = "-"
	}

	fmt.Fprintf(w, "%s - %s %s %d %d %d %s \"%s\" \"%s\" \"%s\"\n",
		req.RemoteAddr, time.Now().Format("2006-01-02T15:04:05"), req.Host,
		msg.Status, msg.Size, msg.Usec,
		req.Method, req.RequestURI,
		logEscape(rfr), logEscape(ua))

}

func verboseLog(dl Logger, msg *Collect) {

	req := msg.Req
	header := req.Header
	rfr := header.Get("Referer")
	ua := header.Get("User-Agent")

	if rfr == "" {
		rfr = "-"
	}
	if ua == "" {
		ua = "-"
	}

	dl.Verbose("%s - %s %s %d %d %d %s \"%s\" \"%s\" \"%s\"",
		req.RemoteAddr, time.Now().Format("2006-01-02T15:04:05"), req.Host,
		msg.Status, msg.Size, msg.Usec,
		req.Method, req.RequestURI,
		logEscape(rfr), logEscape(ua))

}

// ################################################################

// responseWriter to track status+size

func (w *responseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.size += int64(len(b))
	return w.w.Write(b)
}

func (w *responseWriter) WriteHeader(s int) {
	w.status = s
	w.w.WriteHeader(s)
}

// ################################################################
// modeled after net/url escape, but simpler

func shouldEscape(c byte) bool {

	switch c {
	case ' ', '"', '%':
		return true
	}

	if c <= ' ' || c >= 127 {
		return true
	}

	return false
}

func logEscape(s string) string {

	hexCount := 0
	slen := len(s)

	for i := 0; i < slen; i++ {
		if shouldEscape(s[i]) {
			hexCount++
		}
	}

	if hexCount == 0 {
		return s
	}

	t := make([]byte, len(s)+2*hexCount)
	j := 0

	for i := 0; i < slen; i++ {
		c := s[i]
		if shouldEscape(c) {
			t[j] = '%'
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		} else {
			t[j] = c
			j++
		}
	}
	return string(t)
}
