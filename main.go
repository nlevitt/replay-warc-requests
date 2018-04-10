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
	"io/ioutil"
	"bytes"
)

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
			log.Panicln(err, "reading record from", w.name)
		}
		if record.Header.Get("warc-type") == "request" {
			// copy to buffer here because otherwise
			// http.ReadRequest just wraps the input stream which
			// is backed by the warc and we have a race condition
			buf, err := ioutil.ReadAll(record.Content)
			if err != nil {
				log.Panicln(err, "reading content of warc record", record, "from", w.name)
			}

			br := bufio.NewReader(bytes.NewReader(buf))
			req, err := http.ReadRequest(br)
			if err != nil {
				log.Panicln(err, "reading http request from warc record", record)

			}
			req.RequestURI = "" // "RequestURI can't be set in client requests"
			req.URL, err = url.Parse(record.Header.Get("warc-target-uri"))
			if err != nil {
				log.Panicln(err, "parsing", req.URL)
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
			log.Panicln(err, "parsing proxy url", proxy)
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}
	return client
}

func main() {
	var proxyPtr = flag.String("proxy", "", "http proxy url (http://host:port)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] WARCFILE...\n", os.Args[0])
		flag.PrintDefaults()
	}
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
			log.Panicln(err, "opening", flag.Arg(i))
		}
		defer file.Close()
		reader, err := warc.NewReaderMode(file, warc.SequentialMode)
		if err != nil {
			log.Panicln(err, "creating warc reader for", flag.Arg(i))
		}
		warcs[i] = Warc{flag.Arg(i), file, reader}
	}

	// kick off reading warcs and replaying everything
	var wg sync.WaitGroup
	for i := 0; i < len(warcs); i++ {
		wg.Add(1)
		go replayRequests(client, warcs[i], &wg)
	}

	// wait for replay goroutines to finish
	wg.Wait()

	log.Println("all done")
}
