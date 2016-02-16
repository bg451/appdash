package main

// This is a near wholesale copy of the opentracing-go/examples/dapperish.go
// file. The only thing that has been changed is the addition of an appdash
// store, traceapp, and appdash/opentracing Tracer. Everything else is the
// exact same to show that it changing backends is O(1).
// One thing I've noticed is the clash of package names, since there is
// opentracing/opentracing and appdash/opentracing

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	otappdash "sourcegraph.com/sourcegraph/appdash/opentracing"
	"sourcegraph.com/sourcegraph/appdash/traceapp"

	"github.com/opentracing/opentracing-go"
)

func client() {
	reader := bufio.NewReader(os.Stdin)
	for {
		ctx, span := opentracing.BackgroundContextWithSpan(
			opentracing.StartSpan("getInput"))
		// Make sure that global trace tag propagation works.
		span.SetTraceAttribute("User", os.Getenv("USER"))
		span.LogEventWithPayload("ctx", ctx)
		fmt.Print("\n\nEnter text (empty string to exit): ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if len(text) == 0 {
			fmt.Println("Exiting.")
			os.Exit(0)
		}

		span.LogEvent(text)

		httpClient := &http.Client{}
		httpReq, _ := http.NewRequest("POST", "http://localhost:8080/", bytes.NewReader([]byte(text)))
		opentracing.InjectSpan(span, opentracing.GoHTTPHeader, httpReq.Header)
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			span.LogEventWithPayload("error", err)
		} else {
			span.LogEventWithPayload("got response", resp)
		}

		span.Finish()
	}
}

func server() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		serverSpan, err := opentracing.JoinTraceFromHeader(
			"serverSpan", req.Header, opentracing.GlobalTracer())
		if err != nil {
			fmt.Println(err)
			return
		}

		serverSpan.SetTag("component", "server")
		defer serverSpan.Finish()

		fullBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			serverSpan.LogEventWithPayload("body read error", err)
		}

		serverSpan.LogEventWithPayload("got request with body", string(fullBody))
		time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	// Create a recent in-memory store, evicting data after 20s.
	//
	// The store defines where information about traces (i.e. spans and
	// annotations) will be stored during the lifetime of the application. This
	// application uses a MemoryStore store wrapped by a RecentStore with an
	// eviction time of 20s (i.e. all data after 20s is deleted from memory).
	memStore := appdash.NewMemoryStore()
	store := &appdash.RecentStore{
		MinEvictAge: 20 * time.Second,
		DeleteStore: memStore,
	}

	// Start the Appdash web UI on port 8700.
	//
	// This is the actual Appdash web UI -- usable as a Go package itself, We
	// embed it directly into our application such that visiting the web server
	// on HTTP port 8700 will bring us to the web UI, displaying information
	// about this specific web-server (another alternative would be to connect
	// to a centralized Appdash collection server).
	tapp := traceapp.New(nil)
	tapp.Store = store
	tapp.Queryer = memStore
	log.Println("Appdash web UI running on HTTP :8700")
	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()

	// We will use a local collector (as we are running the Appdash web UI
	// embedded within our app).
	//
	// A collector is responsible for collecting the information about traces
	// (i.e. spans and annotations) and placing them into a store. In this app
	// we use a local collector (we could also use a remote collector, sending
	// the information to a remote Appdash collection server).
	collector := appdash.NewLocalCollector(store)

	recorder := appdash.NewRecorder(appdash.SpanID{}, collector)
	opentracing.InitGlobalTracer(otappdash.NewTracer(recorder))

	go server()
	go client()

	runtime.Goexit()
}
