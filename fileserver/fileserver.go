package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
)

var (
	root                 = flag.String("root", "/tmp/files", "Root directory for serving files")
	addr                 = flag.String("addr", "", "Address to bind to")
	port                 = flag.Int("port", 80, "port to listen on")
	logFile              = flag.String("log-file", "/var/log/access.log", "Log file to output to")
	templateDir          = flag.String("templates", "/opt/templates/", "directory containing templates")
	postDB               = flag.String("postdb", "", "Database file path")
	passFile             = flag.String("passfile", "", "Password file")
	outLog      *os.File = nil
)

func init() {
	flag.Parse()
	if *root == "" {
		fmt.Printf("ERROR: I need a root directory to serve files from\n")
		os.Exit(-1)
	}
	if *port <= 0 || *port >= 0xffff {
		fmt.Printf("ERROR: I need a usable port to serve on (0 > port > %d)\n", 0xffff)
		os.Exit(-1)
	}
	if *postDB == "" {
		fmt.Printf("ERROR: I need a post DB path\n")
		os.Exit(-1)
	}
	if *passFile == "" {
		fmt.Printf("ERROR: I need a password file\n")
		os.Exit(-1)
	}
	if *port != 0 {
		*addr = fmt.Sprintf("%s:%d", *addr, *port)
	}

	fi, err := os.Stat(*root)
	if err != nil {
		fmt.Printf("ERROR: %s does not exist\t%v\n", *root, err)
		os.Exit(-1)
	}
	if !fi.IsDir() {
		fmt.Printf("ERROR: %s is not a directory\n", *root)
		os.Exit(-1)
	}
	outLog, err = os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0660)
	if err != nil {
		fmt.Printf("ERROR opening the log file %v\t%v\n", *logFile, err)
		os.Exit(-1)
	}
	_, err = outLog.Seek(0, 2)
	if err != nil {
		fmt.Printf("Error seeking to end of log file\n")
		os.Exit(-1)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		if outLog != nil {
			outLog.Close()
		}
		ClosePostDB()
		os.Exit(0)
	}()
}

func main() {
	defer outLog.Close()

	bts, err := ioutil.ReadFile(*passFile)
	if err != nil {
		fmt.Printf("Failed to read password file: %v\n", err)
		return
	}
	if err := SetUpdatePassbytes(bts); err != nil {
		fmt.Printf("Failed to set password: %v\n", err)
		return
	}

	if err := InitPostDB(*postDB); err != nil {
		fmt.Printf("Failed to init post DB: %v\n", err)
		return
	}
	defer ClosePostDB()

	//grab handle on listener
	lst, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Printf("Failed to get listener: %v\n", err)
		return
	}

	if err := SetMainTemplateFile(*templateDir + "/main.template"); err != nil {
		fmt.Printf("Failed to find main template: %v\n", err)
		return
	}
	mux := http.NewServeMux()
	dirs := []string{"/files/", "/js/", "/css/", "/fonts"}
	for i := range dirs {
		mux.Handle(dirs[i], LogAndServe(http.StripPrefix(dirs[i], http.FileServer(http.Dir(*root+dirs[i])))))
	}
	mux.HandleFunc("/", templateHandler)
	mux.HandleFunc("/update", postUpdateHandler)
	panic(http.Serve(lst, mux))
}
