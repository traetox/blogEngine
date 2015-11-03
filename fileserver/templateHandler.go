package main

import (
	"net/http"
	"text/template"
	"os"
	"path"
)

var (
	mainTemplateFile string
)

func SetMainTemplateFile(file string) error {
	fin, err := os.Open(file)
	if err != nil {
		return err
	}
	if err := fin.Close(); err != nil {
		return err
	}
	mainTemplateFile = file
	return nil
}

func templateHandler(w http.ResponseWriter, r *http.Request) {
	rc := NewResponseCapture(w)
	if r.Method != "GET" {
		rc.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/" {
		if err := getLatest(rc); err != nil {
			rc.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		_, req := path.Split(path.Clean(r.URL.Path))
		if err := getUpdate(rc, req); err != nil {
			rc.WriteHeader(http.StatusInternalServerError)
		}
	}
	//always log the request
	logRequest(r, rc.Code())
}

type BlogPost struct {
	Title string
	Date  string
	Content string
}

func getLatest(rc *ResponseCapture) error {
	t, err := template.ParseFiles(mainTemplateFile)
	if err != nil {
		return err
	}
	bp := BlogPost {
		Title: "hello!",
		Date: "Right bout now",
		Content: "testing HTML stuff<br>and things<br>",
	}
	if err := t.Execute(rc, bp); err != nil {
		return err
	}
	return nil
}

func getUpdate(rc *ResponseCapture, req string) error {
	return nil
}
