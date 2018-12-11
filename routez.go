// Copyright (c) 2018
// Author: Jeff Weisberg <jaw @ tcp4me.com>
// Created: 2018-Dec-11 15:40 (EST)
// Function: trivial request router

package enginz

import (
	"fmt"
	"net/http"
)

// Routes implements a trivial http router
//
// var router = enginz.Routes{
//     "/hello": hello,
//     "/1.gif": enginz.BlankGif,
//     "_404":   notFound,
// }
//
// func hello(w http.ResponseWriter, req *http.Request) { ... }
//
// it must not be modified while the server is running
//
type Routes map[string]HandlerFunc

func (r Routes) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	path := req.URL.Path
	f, ok := r[path]

	if ok {
		f(w, req)
		return
	}

	// 404 not found
	f, ok = r["_404"]

	if ok {
		f(w, req)
		return
	}

	w.WriteHeader(404)
	fmt.Fprintf(w, "File Not Found. So Sorry.\n")
}
