package main

import (
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
	"io"
	"log"
	"net/http"

	"go.opentelemetry.io/otel/api/correlation"
	"go.opentelemetry.io/otel/api/global"
	//"go.opentelemetry.io/otel/exporters/trace/stdout"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/plugin/httptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() {
	// Create stdout exporter to be able to retrieve
	// the collected spans.
	//exporter, err := stdout.NewExporter(stdout.Options{PrettyPrint: true})
	exporter, err := jaeger.NewRawExporter(
		jaeger.WithCollectorEndpoint("http://localhost:14268/api/traces"),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: "server",
			Tags: []core.KeyValue{
				key.String("exporter", "jaeger"),
				key.String("type", "http"),
			},
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// For the demonstration, use sdktrace.AlwaysSample sampler to sample all traces.
	// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	if err != nil {
		log.Fatal(err)
	}
	global.SetTraceProvider(tp)
}

func helloHandler(w http.ResponseWriter, req *http.Request) {
	tracer := global.TraceProvider().Tracer("example/server")

	// Extracts the conventional HTTP span attributes,
	// distributed context tags, and a span context for
	// tracing this request.
	attrs, entries, spanCtx := httptrace.Extract(req.Context(), req)
	ctx := req.Context()
	if spanCtx.IsValid() {
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanCtx)
	}

	// Apply the correlation context tags to the request
	req = req.WithContext(correlation.ContextWithMap(ctx, correlation.NewMap(correlation.MapUpdate{
		MultiKV: entries,
	})))

	// Start the server-side span, passing the remote
	// child span context explicitly.
	_, span := tracer.Start(
		req.Context(),
		"hello",
		trace.WithAttributes(attrs...),
	)
	defer span.End()

	io.WriteString(w, "Hello, world!\n")
}

func main() {
	initTracer()

	http.HandleFunc("/hello", helloHandler)
	err := http.ListenAndServe(":7777", nil)
	if err != nil {
		panic(err)
	}
}
