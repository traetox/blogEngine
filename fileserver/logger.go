package main

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

type ResponseCapture struct {
	w    http.ResponseWriter
	code int
}

func (rc *ResponseCapture) Header() http.Header {
	return rc.w.Header()
}

func (rc *ResponseCapture) Write(b []byte) (int, error) {
	return rc.w.Write(b)
}

func (rc *ResponseCapture) WriteHeader(c int) {
	rc.code = c
	rc.w.WriteHeader(c)
}

func (rc *ResponseCapture) Code() int { return rc.code }

func NewResponseCapture(rw http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		w:    rw,
		code: 200,
	}
}

func LogAndServe(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := NewResponseCapture(w)
		handler.ServeHTTP(rc, r)
		logRequest(r, rc.Code())
	})
}

func logRequest(r *http.Request, response int) {
	if outLog == nil {
		fmt.Printf("not outLog\n")
		return
	}
	t := time.Now()
	tmStr := t.Format("2006-01-02 15:04")
	nano := t.Nanosecond() / 1000000
	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		addr = r.RemoteAddr
	}
	n, err := fmt.Fprintf(outLog, "%s.%03d %s %s %s%s %d [%s]\n", tmStr, nano, addr, r.Method, r.Host, r.URL, response, r.UserAgent())
	if err != nil {
		fmt.Printf("Log error: %v\n", err)
	}
	if n == 0 {
		fmt.Printf("Log error, wrote 0 bytes\n")
	}
}
