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

func replayRequest(client *http.Client, record *warc.Record) {
	reader := bufio.NewReader(record.Content)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	req.RequestURI = "" // "RequestURI can't be set in client requests"
	req.URL, err = url.Parse(record.Header.Get("warc-target-uri"))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	buf := make([]byte, 65536)
	size := 0
	for {
		n, err := res.Body.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		size += n
	}
	log.Printf("%v (%v bytes) %v %v\n", res.Status, size, req.Method, req.URL)
}

func replayRequests(client *http.Client, w Warc, wg sync.WaitGroup) {
	defer wg.Done()

	for {
		record, err := w.reader.ReadRecord()
		if err == io.EOF {
			log.Println("finished!")
			os.Exit(0)
		}
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		if record.Header.Get("warc-type") == "request" {
			replayRequest(client, record)
		}
	}
}

func main() {
	var proxyPtr = flag.String("proxy", "", "http proxy host:port")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	log.Println("proxy:", *proxyPtr)

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
	if *proxyPtr != "" {
		proxyUrl, err := url.Parse(*proxyPtr)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}
	log.Printf("client.Transport: %+v", client.Transport)

	// open warcs for reading
	warcs := make([]Warc, flag.NArg())
	for i := 0; i < flag.NArg(); i++ {
		file, err := os.Open(flag.Arg(i))
		if err != nil {
		}
		defer file.Close()
		reader, err := warc.NewReader(file)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		warcs[i] = Warc{flag.Arg(i), file, reader}
		log.Printf("warcs[%d]: %v", i, warcs[i])
	}

	var wg sync.WaitGroup
	for i := 0; i < len(warcs); i++ {
		wg.Add(1)
		go replayRequests(client, warcs[i], wg)
	}
	wg.Wait()
	log.Println("all done")
}
