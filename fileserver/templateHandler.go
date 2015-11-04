package main

import (
	"crypto/sha512"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path"
	"text/template"
)

var (
	mainTemplateFile string
	db               *boltDB
	lastRand         int64
	lastRandReqAddr  string
	password         string

	errNotAuthorized = errors.New("not authorized")
)

func SetUpdatePassword(pass string) error {
	if password != "" {
		return errors.New("Already set")
	}
	password = pass
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
		fmt.Fprintf(w, "%d", lastRand)
	case "PUT":
		if err := decodeNewUpdate(lastRand, lastRandReqAddr, r); err != nil {
			rc.WriteHeader(http.StatusInternalServerError)
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
			bp = BlogPost{
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
	return nil
}

type newBlogPost struct {
	Auth []byte
	Name string
	BP   BlogPost
}

func decodeNewUpdate(seed int64, lastSeedAddr string, r *http.Request) error {
	var nbp newBlogPost
	if seed == 0 || lastSeedAddr == "" || password == "" {
		return errors.New("no seed set")
	}
	defer r.Body.Close()
	jdec := json.NewDecoder(r.Body)
	if err := jdec.Decode(&nbp); err != nil {
		return err
	}
	//check if the authentication checks out
	res := hashIt(seed, password)

	//check out out
	if len(res) != len(nbp.Auth) {
		return errNotAuthorized
	}
	for i := 0; i < len(res); i++ {
		if res[i] != nbp.Auth[i] {
			return errNotAuthorized
		}
	}
	//it checks out
	if db != nil {
		return db.Add(nbp.Name, &nbp.BP)
	}
	return errors.New("db is not ready")
}

func hashIt(seed int64, pass string) []byte {
	buff := make([]byte, 8)
	binary.PutVarint(buff, seed)
	hsh := sha512.New()
	hsh.Write(buff)
	hsh.Write([]byte(password))
	hsh.Write(buff)
	res := hsh.Sum(nil)
	for i := 0; i < 256; i++ {
		res = hsh.Sum(res)
	}
	return res
}
