package main

import (
	"flag"
	"os"
	"github.com/datatogether/warc"
	"log"
	// "io"
	"fmt"
)

func usage() {
	fmt.Printf("Usage: %s [OPTIONS] WARCFILE...\n", os.Args[0])
	flag.PrintDefaults()
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

	// var warcFile = flag.Arg(0);
	// log.Println("warcFile:", warcFile)

	// open warcs for reading
	readers := make([]*warc.Reader, flag.NArg())
	for i := 0; i < flag.NArg(); i++ {
		f, err := os.Open(flag.Arg(i))
		if err != nil {
		}
		defer f.Close()
		r, err := warc.NewReader(f)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		readers[i] = r
		log.Printf("readers[%d]: %v", i, readers[i])
	}

	// iterate over records
	/*
	for {
		record, err := r.Read()
		if err == io.EOF {
			log.Println("finished!")
			os.Exit(0)
		}
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		log.Println("record:", record)
	}
	*/

	/*
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	log.Println("len(records):", len(records))
	*/


}

