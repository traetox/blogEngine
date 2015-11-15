package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"../blogpost"
)

var (
	passfile     = flag.String("passfile", "", "Password file")
	addr         = flag.String("a", "", "Address of blog server")
	templateFile = flag.String("f", "", "Template file")
	name         = flag.String("n", "", "Name of new post")
	title        = flag.String("t", "", "Title of new post")
)

func init() {
	flag.Parse()
	if *passfile == "" {
		log.Fatal("Passfile required")
	}
	if *addr == "" {
		log.Fatal("Server address required")
	}
	if *templateFile == "" {
		log.Fatal("Template file required")
	}
	if *name == "" {
		log.Fatal("Name required")
	}
	if *title == "" {
		log.Fatal("Title required")
	}
}

func getSeed(addr string) (int64, error) {
	var seed int64
	res, err := http.Get(addr + "/update")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if err := binary.Read(res.Body, binary.LittleEndian, &seed); err != nil {
		return -1, err
	}
	return seed, nil
}

func pushPost(addr string, seed int64, passbytes []byte, name string, bp blogpost.BlogPost) error {
	bb := bytes.NewBuffer(nil)
	if err := blogpost.WriteBlogPost(bb, bp, name, seed, passbytes); err != nil {
		return err
	}
	resp, err := http.Post(addr+"/update", "application/json", bb)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("Bad status: " + resp.Status)
	}
	return nil
}

func main() {
	passbytes, err := ioutil.ReadFile(*passfile)
	if err != nil {
		log.Fatal("Failed to read", *passfile)
	}
	templatebytes, err := ioutil.ReadFile(*templateFile)
	if err != nil {
		log.Fatal("Failed to read", *templateFile)
	}

	//go get the seed
	seed, err := getSeed(*addr)
	if err != nil {
		log.Fatal("Failed to get seed from", *addr, err)
	}

	bp := blogpost.BlogPost{
		Title:   *title,
		Date:    time.Now(),
		Content: string(templatebytes),
	}

	//push the hash package
	if err := pushPost(*addr, seed, passbytes, *name, bp); err != nil {
		log.Fatal("Failed to push package", err)
	}
	log.Println("New post pushed")
}
