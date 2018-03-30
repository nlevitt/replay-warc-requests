# replay-warc-requests

Reads request records from warc files and replays them.

Initially designed for performance testing warcprox under realistic load.

## install

    go get github.com/nlevitt/replay-warc-requests
 
## run

    replay-warc-requests --proxy=http://localhost:8000 my.warc.gz other.warc.gz
  
