package main

import (
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func serviceName() string{
	if name := os.Getenv("NAME"); name != "" {
		return name
	}else{
		return  "unknown"
	}
}

func initTracer() (opentracing.Tracer, io.Closer) {
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName(),
		Sampler: &jaegercfg.SamplerConfig{
			Type:	"const",
			Param:	1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:		true,
			BufferFlushInterval:	1 * time.Second,
			LocalAgentHostPort: fmt.Sprintf("%s:%s", os.Getenv("JAEGER_AGENT_HOST"), os.Getenv("JAEGER_AGENT_PORT")),
		},
	}
	tracer, closer, err := cfg.NewTracer(jaegercfg.Logger(jaeger.StdLogger))
	if err != nil {
		log.Panicf("Could not initialize jaeger tracer: %s", err.Error())
	}
	return tracer, closer
}

func sendOutgoingRequest(nextUrl string, parentSpan opentracing.Span) string {
	nextUrlSpan := opentracing.StartSpan("outgoing_request", ext.RPCServerOption(parentSpan.Context()))
	nextUrlSpan.LogKV("path", nextUrl)
	defer nextUrlSpan.Finish()

	httpClient := &http.Client{}
	httpReq, _ := http.NewRequest("GET", nextUrl, nil)

	opentracing.GlobalTracer().Inject(
		parentSpan.Context(),
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(httpReq.Header))

	resp, err := httpClient.Do(httpReq)
	nextUrlSpan.LogKV("http.status", strconv.Itoa(resp.StatusCode))

	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		return string(bodyBytes)
	}else{
		nextUrlSpan.SetTag("error", true)
		return "Empty response"
	}
}

func main() {
	tracer, closer := initTracer()
	defer closer.Close()

	opentracing.SetGlobalTracer(tracer)

	http.HandleFunc("/", func (w http.ResponseWriter, r *http.Request) {
		if rand.Intn(10) == 1 {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Server error")
		}else{
			var span opentracing.Span
			wireContext, err := opentracing.GlobalTracer().Extract(
				opentracing.HTTPHeaders,
				opentracing.HTTPHeadersCarrier(r.Header))

			if err != nil {
				log.Printf("Span not serealized: %s", err.Error())
				span = opentracing.StartSpan("incoming_request")
			}else{
				span = opentracing.StartSpan("incoming_request", ext.RPCServerOption(wireContext))
			}

			span.LogKV("path", "/")

			defer span.Finish()

			if nextUrl := os.Getenv("NEXT_URL"); nextUrl != "" {
				body := sendOutgoingRequest(nextUrl, span)
				fmt.Fprintf(w, body)
			}else{
				fmt.Fprintf(w, "Empty NEXT_URL")
			}
		}
	})

	if port := os.Getenv("PORT"); port != "" {
		http.ListenAndServe(":"+port, nil)
	}else{
		http.ListenAndServe(":80", nil)
	}
}
