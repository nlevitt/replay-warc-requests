package main

import (
	"flag"
	"fmt"
	"github.com/slyrz/warc"
	"io"
	"log"
	"os"
	"sync"
	"bufio"
	"net/http"
	"net/url"
	"crypto/tls"
	"time"
	"runtime"
	"io/ioutil"
	"runtime/pprof"
)

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] WARCFILE...\n", os.Args[0])
	flag.PrintDefaults()
}

type Warc struct {
	name   string
	file   *os.File
	reader *warc.Reader
}

func replayRequest(client *http.Client, req *http.Request, w Warc, done chan bool) {
	defer func() { done <- true }()

	response, err := client.Do(req)
	if err != nil {
		log.Println(err, "requesting", req.URL)
		return
	}
	defer response.Body.Close()

	size, err := io.Copy(ioutil.Discard, response.Body)
	if err != nil {
		log.Println(err, "downloading", req.URL)
		return
	}
	log.Printf("%v (%v bytes) %v %v %v\n", response.Status, size,
		req.Method, req.URL, w.name)
}

func replayRequests(client *http.Client, w Warc, wg *sync.WaitGroup) {
	defer wg.Done()

	activeRequests := 0
	requestDone := make(chan bool)

	for {
		record, err := w.reader.ReadRecord()
		if err == io.EOF {
			break // end of warc
		}
		if err != nil {
			log.Fatalln(err, "reading record from", w.name)
		}
		if record.Header.Get("warc-type") == "request" {
			reader := bufio.NewReader(record.Content)
			req, err := http.ReadRequest(reader)
			if err != nil {
				log.Fatalln(err, "reading http request from warc record", record)
			}
			req.RequestURI = "" // "RequestURI can't be set in client requests"
			req.URL, err = url.Parse(record.Header.Get("warc-target-uri"))
			if err != nil {
				log.Fatalln(err, "parsing", req.URL)
			}

			if activeRequests >= 6 {
				// wait for an outstanding request to finish
				<-requestDone
				activeRequests--
			}
			activeRequests++
			go replayRequest(client, req, w, requestDone)
		}
	}
	// wait for outstanding requests
	for activeRequests > 0 {
		<-requestDone
		activeRequests--
	}
	log.Println("finished replaying", w.name)
}

func httpClient(proxy string) (*http.Client) {
	// "Clients are safe for concurrent use by multiple goroutines."
	transport :=  &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		// https://stackoverflow.com/questions/23297520
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: transport,
		Jar: nil,
	}
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			log.Fatalln(err, "parsing proxy url", proxy)
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}
	return client
}

func main() {
	var proxyPtr = flag.String("proxy", "", "http proxy url (http://host:port)")
	var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	client := httpClient(*proxyPtr)

	// open warcs for reading
	warcs := make([]Warc, flag.NArg())
	for i := 0; i < flag.NArg(); i++ {
		file, err := os.Open(flag.Arg(i))
		if err != nil {
			log.Fatalln(err, "opening", flag.Arg(i))
		}
		defer file.Close()
		reader, err := warc.NewReaderMode(file, warc.SequentialMode)
		if err != nil {
			log.Fatalln(err, "creating warc reader for", flag.Arg(i))
		}
		warcs[i] = Warc{flag.Arg(i), file, reader}
	}

	// kick off reading warcs and replaying everything
	var wg sync.WaitGroup
	for i := 0; i < len(warcs); i++ {
		wg.Add(1)
		go replayRequests(client, warcs[i], &wg)
	}

	// periodic stats reporting
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			// log.Println("before GC:", runtime.NumGoroutine(), "goroutines")
			// runtime.GC()
			// log.Println("after GC:", runtime.NumGoroutine(), "goroutines")
			log.Println(runtime.NumGoroutine(), "goroutines")
			buf := make([]byte, 65536)
			var n int
			for {
				n = runtime.Stack(buf, true)
				if n >= len(buf) {
					buf = make([]byte, 2*len(buf))
				} else {
					break
				}
			}
			log.Println(n, "bytes of stack")
			os.Stderr.Write(buf[:n])
			// log.Println(buf[:n])

			if *memprofile != "" {
				f, err := os.Create(*memprofile)
				if err != nil {
					log.Fatal("could not create memory profile: ", err)
				}
				runtime.GC() // get up-to-date statistics
				if err := pprof.WriteHeapProfile(f); err != nil {
					log.Fatal("could not write memory profile: ", err)
				}
				f.Close()
			}
		}
	}()

	// wait for replay goroutines to finish
	wg.Wait()

	ticker.Stop()
	log.Println("all done")
}
