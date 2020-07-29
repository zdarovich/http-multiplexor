package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type (
	request struct {
		Urls []string `json:"urls,omitempty"`
	}

	data struct {
		Body  []byte `json:"body,omitempty"`
		Error error `json:"error,omitempty"`
	}
)


func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: maxClients(mux, 100),
	}


	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Print("Server Started")

	<-done
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		// extra handling here
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited Properly")
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		statusStatusBadRequest(w, r, "Wrong path")
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		statusStatusBadRequest(w, r, "Wrong body")
		return
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		statusStatusBadRequest(w, r, "Wrong body")
		return
	}

	if len(req.Urls) > 20 {
		statusStatusBadRequest(w, r, "Only 20 urls allowed")
		return
	}

	done := make(chan interface{})
	defer close(done)

	urls := prepare(done, req.Urls)

	fetchers := make([]<-chan interface{}, 4)
	for i := 0; i < 4; i++ {
		fetchers[i] = fetch(done, urls, i)
	}

	var datas []data
	for val := range orDone(done, merge(done, fetchers...)) {
		datas = append(datas, val.(data))
	}
	if b, err := json.Marshal(datas); err != nil {
		w.WriteHeader(500)
	} else {
		w.Write(b)
	}
}

func maxClients(h http.Handler, n int) http.Handler {
	sema := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeOut := time.NewTimer(time.Millisecond * 1200)
		select {
		case sema <- struct{}{}:
			h.ServeHTTP(w, r)
			defer func() { <-sema }()
		case <-timeOut.C:
			statusServiceUnavailable(w, r)
		}

	})
}

func prepare(done <-chan interface{}, urls []string) <-chan string {
	urlsChan := make(chan string)

	go func() {
		defer close(urlsChan)

		for _, url := range urls {
			select {
			case <-done:
				return
			case urlsChan <- url:
			}
		}
	}()
	return urlsChan
}

func fetch(done chan interface{}, urls <-chan string, fetcherID int) <-chan interface{} {
	resps := make(chan interface{})

	go func() {
		defer close(resps)

		for url := range urls {
			select {
			case <-done:
				return
			default:
				var body []byte
				var err error
				fmt.Printf("Fetcher #%d: Url: %s started\n", fetcherID, url)

				cli := http.Client{
					Timeout: 1 * time.Second,
				}

				resp, err := cli.Get(url)
				if err != nil {
					resps <- data{Body: body, Error: err}
					return
				}

				body, err = ioutil.ReadAll(resp.Body)

				resps <- data{Body: body, Error: err}
				resp.Body.Close()

			}
		}
	}()
	return resps
}

func merge(done <-chan interface{}, channels ...<-chan interface{}) <-chan interface{} {
	var wg sync.WaitGroup
	multiplexedStream := make(chan interface{})
	multiplex := func(c <-chan interface{}) {
		defer wg.Done()
		for i := range c {
			select {
			case <-done:
				return
			case multiplexedStream <- i:
			}
		}
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go multiplex(c)
	}
	go func() {
		wg.Wait()
		close(multiplexedStream)
	}()
	return multiplexedStream
}

func orDone(done, c <-chan interface{}) <-chan interface{} {
	valStream := make(chan interface{})
	go func() {
		defer close(valStream)
		for {
			select {
			case <-done:
				return
			case v, ok := <-c:
				if ok == false {
					return
				}
				select {
				case valStream <- v:
				case <-done:
				}
			case <-time.After(10 * time.Second):
				return
			}
		}
	}()
	return valStream
}


func statusStatusBadRequest(w http.ResponseWriter, r *http.Request, msg string) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(msg))
}
func statusServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("Too many simultaneous requests"))
}