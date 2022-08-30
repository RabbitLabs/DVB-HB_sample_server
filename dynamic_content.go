package main

import (
	"net/http"
	"strings"
)

const DynamicContentPath = "/dynamic/"

func RegisterDynamicContent(name string, m DynamicContent) {
	deviceconfig.dynamiccontent[name] = m
}

func dynamicContentHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, DynamicContentPath) {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}

	subpath := r.URL.Path[len(DynamicContentPath):]
	subpath = strings.TrimLeft(subpath, "/")

	splitpath := strings.SplitN(subpath, "/", 2)

	// we must have two part path
	if len(splitpath) != 2 {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	// try to find channel map
	dynamicchannelmap, exists := deviceconfig.dynamiccontent[splitpath[0]]

	if !exists {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	dynamicchannelmap.ServeDynamicContent(w, r, splitpath[1])
}
