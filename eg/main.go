// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Dec-11 15:55 (EST)
// Function: example

package main

import (
	"fmt"
	"net/http"

	"github.com/jaw0/enginz"
	"github.com/jaw0/acgo/diag"
)

var router = enginz.Routes{
	"/hello": hello,
	"/gif":   enginz.BlankGif,
	"_404":   notFound,
}

var dl = diag.Logger("main")

func main() {

	// read config
	// diag SetConfig, Init

	http := &enginz.Server{
		Service: []enginz.Service{
			{Addr: ":8080"},
			{Addr: ":8081"},
			{Addr: ":8443", TLSKey: "/Users/jaw/src/certstrap/out/127.0.0.1.key", TLSCert: "/Users/jaw/src/certstrap/out/127.0.0.1.crt"},
		},
		Handler:   router,
		Report:    diag.Logger("http"),
		AccessLog: "/tmp/access.log",
		ErrorLog:  "/tmp/error.log",
		TraceID:   "ccsphl-dev-1234",
	}

	http.Serve()
}

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello, world!\n")
}

func notFound(w http.ResponseWriter, req *http.Request) {

	w.WriteHeader(404)
	fmt.Fprintf(w, "I cannot find your file. so sorry.\n")
}
