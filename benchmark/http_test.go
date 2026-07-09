package benchmark

import (
	"io"
	"log"
	"net/http"
	"testing"
)

// http 压测工具
// 系统自带（自身可能成性能瓶颈） ab -n 10000 -c 100 http://127.0.0.1:9090
// 专业（brew install wrk） wrk -t10 -c100 -d30s http://127.0.0.1:9090/
/*
	Running 30s test @ http://127.0.0.1:9090/
  10 threads and 100 connections
  Thread Stats   Avg      Stdev     Max   +/- Stdev
    Latency   718.93us    0.91ms  18.81ms   90.43%
    Req/Sec    14.21k     2.03k   35.84k    75.73%
  4247602 requests in 30.10s, 518.51MB read
	Requests/sec: 141118.98
	Transfer/sec:     17.23MB
*/

func sayhello(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	io.WriteString(w, "hello world")
}

func TestHttpServer(t *testing.T) {
	http.HandleFunc("/", sayhello)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
