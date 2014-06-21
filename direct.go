package mallory

import (
	"io"
	"net/http"
	"time"
)

// Direct fetcher from the host of proxy
type EngineDirect struct {
	Tr *http.Transport
}

// Create and initialize
func CreateEngineDirect(e *Env) (*EngineDirect, error) {
	return &EngineDirect{Tr: http.DefaultTransport.(*http.Transport)}, nil
}

// Data flow:
//  1. Receive request R1 from client
//  2. Re-post request R1 to remote server(the one client want to connect)
//  3. Receive response P1 from remote server
//  4. Send response P1 to client
func (self *EngineDirect) Serve(s *Session) {
	w, r := s.ResponseWriter, s.Request
	if r.Method == "CONNECT" {
		s.Error("this function can not handle CONNECT method")
		return
	}
	start := time.Now()

	// Client.Do is different from DefaultTransport.RoundTrip ...
	// Client.Do will change some part of request as a new request of the server.
	// The underlying RoundTrip never changes anything of the request.
	resp, err := self.Tr.RoundTrip(r)
	if err != nil {
		s.Error("RoundTrip: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	// please prepare header first and write them
	CopyHeader(w, resp)
	w.WriteHeader(resp.StatusCode)

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		s.Error("Copy: %s", err.Error())
		return
	}

	d := BeautifyDuration(time.Since(start))
	ndtos := BeautifySize(n)
	s.Info("RESPONSE %s %s in %s <-%s", r.URL.Host, resp.Status, d, ndtos)
}

// Data flow:
//  1. Receive CONNECT request from the client
//  2. Dial the remote server(the one client want to conenct)
//  3. Send 200 OK to client if the connection is established
//  4. Exchange data between client and server
func (self *EngineDirect) Connect(s *Session) {
	w, r := s.ResponseWriter, s.Request
	if r.Method != "CONNECT" {
		s.Error("this function can only handle CONNECT method")
		return
	}
	start := time.Now()

	// Use Hijacker to get the underlying connection
	hij, ok := w.(http.Hijacker)
	if !ok {
		s.Error("Server does not support Hijacker")
		return
	}

	src, _, err := hij.Hijack()
	if err != nil {
		s.Error("Hijack: %s", err.Error())
		return
	}
	defer src.Close()

	// connect the remote client directly
	dst, err := self.Tr.Dial("tcp", r.URL.Host)
	if err != nil {
		s.Error("Dial: %s", err.Error())
		return
	}
	defer dst.Close()

	// Once connected successfully, return OK
	src.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	// Proxy is no need to know anything, just exchange data between the client
	// the the remote server.
	copyAndWait := func(w io.Writer, r io.Reader, c chan int64) {
		n, err := io.Copy(w, r)
		if err != nil {
			s.Error("Copy: %s", err.Error())
		}
		c <- n
	}

	// client to remote
	stod := make(chan int64)
	go copyAndWait(dst, src, stod)

	// remote to client
	dtos := make(chan int64)
	go copyAndWait(src, dst, dtos)

	// Generally, the remote server would keep the connection alive,
	// so we will not close the connection until both connection recv
	// EOF and are done!
	nstod, ndtos := BeautifySize(<-stod), BeautifySize(<-dtos)
	d := BeautifyDuration(time.Since(start))
	s.Info("CLOSE %s after %s ->%s <-%s", r.URL.Host, d, nstod, ndtos)
}
