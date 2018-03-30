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

func replayRequest(record *warc.Record, proxy string) {
	record.Header.Get("warc-target-uri")
	reader := bufio.NewReader(record.Content)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	fmt.Println("request:", req)
}

func replayRequests(w Warc, proxy string, wg sync.WaitGroup) {
	defer wg.Done()
	for {
		// offset, err := w.file.Seek(0, os.SEEK_CUR)
		// if err == io.EOF {
		// 	log.Fatal(err)
		// 	os.Exit(1)
		// }
		record, err := w.reader.ReadRecord()
		if err == io.EOF {
			log.Println("finished!")
			os.Exit(0)
		}
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		/*
		log.Printf("offset=%v len(headers)=%v type=%v url=%v warc=%v\n",
			offset, len(record.Header),
			record.Header.Get("warc-type"),
			record.Header.Get("warc-target-uri"),
			w.name)
			*/
		if record.Header.Get("warc-type") == "request" {
			replayRequest(record, proxy)
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
		go replayRequests(warcs[i], *proxyPtr, wg)
	}
	wg.Wait()
	log.Println("all done")
}
