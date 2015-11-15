package main

import (
	"encoding/binary"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"path"
	"text/template"

	"../blogpost"
)

var (
	mainTemplateFile string
	db               *boltDB
	lastRand         int64
	lastRandReqAddr  string
	passbytes        []byte

	errNotAuthorized = errors.New("not authorized")
	errNilDB         = errors.New("Nil DB")
)

func SetUpdatePassbytes(pass []byte) error {
	if passbytes != nil {
		return errors.New("Already set")
	}
	passbytes = pass
	return nil
}

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

func InitPostDB(boltDbFile string) error {
	if db != nil {
		return errors.New("DB already open")
	}
	tdb, err := NewBlogDB(boltDbFile)
	if err != nil {
		return err
	}
	db = tdb
	return nil
}

func ClosePostDB() error {
	if db == nil {
		return errors.New("DB already closed")
	}
	if err := db.Close(); err != nil {
		return err
	}
	db = nil
	return nil
}

func postUpdateHandler(w http.ResponseWriter, r *http.Request) {
	rc := NewResponseCapture(w)
	switch r.Method {
	case "GET":
		lastRandReqAddr = r.RemoteAddr
		lastRand = rand.Int63()
		if err := binary.Write(w, binary.LittleEndian, lastRand); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	case "POST":
		if err := decodeNewUpdate(lastRand, lastRandReqAddr, r); err != nil {
			rc.WriteHeader(http.StatusForbidden)
		}
		lastRand = 0
	default:
		rc.WriteHeader(http.StatusMethodNotAllowed)
	}
	//always log the request
	logRequest(r, rc.Code())
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

func getLatest(rc *ResponseCapture) error {
	t, err := template.ParseFiles(mainTemplateFile)
	if err != nil {
		return err
	}
	bp, err := db.LatestPost()
	if err != nil {
		if err == errNoPosts {
			bp = blogpost.BlogPost{
				Title: "Nothing here yet",
			}
		} else {
			return err
		}
	}
	if err := t.Execute(rc, bp); err != nil {
		return err
	}
	return nil
}

func getUpdate(rc *ResponseCapture, req string) error {
	t, err := template.ParseFiles(mainTemplateFile)
	if err != nil {
		return err
	}
	bp, err := db.Get(req)
	if err != nil {
		if err == errNotFound {
			custom404(rc)
			return nil
		} else {
			return err
		}
	}
	if err := t.Execute(rc, bp); err != nil {
		return err
	}
	return nil
}

func decodeNewUpdate(seed int64, lastSeedAddr string, r *http.Request) error {
	defer r.Body.Close()
	if seed == 0 || lastSeedAddr == "" || passbytes == nil {
		return errors.New("no seed set")
	}
	var bp blogpost.BlogPost
	var name string
	if err := blogpost.ReadBlogPost(r.Body, &bp, &name, seed, passbytes); err != nil {
		return err
	}

	if err := db.Add(name, &bp); err != nil {
		return err
	}
	return nil
}

func custom404(rc *ResponseCapture) {
	rc.WriteHeader(http.StatusNotFound)
	b := []byte(`<h1>Error 404<h1>
		  <h4>There is nothing to see here</h4>
		  <h5>No, really, I couldn't find anything.</h5>`)
	rc.Write(b)
}
