package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"go.opentelemetry.io/otel/codes"
	"math/rand"
)

func initTracer() *sdktrace.TracerProvider {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil
	}
	
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("servico2-orquestrador"), 
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res), 
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	
	return tp
}

func tarefaPesadaHandler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("servico2-orquestrador")
	_, span := tracer.Start(r.Context(), "Processar-Logica")
	defer span.End()

	fmt.Println("-> Serviço 2: A processar o pedido...")
	time.Sleep(200 * time.Millisecond) // Simula 200ms de trabalho

	if rand.Intn(10) < 3 {
		fmt.Println("   [!] CAOS: A injetar falha aleatória...")
		span.SetStatus(codes.Error, "Falha aleatória na Base de Dados")
		span.RecordError(fmt.Errorf("erro crítico ao aceder aos dados (Simulação de Caos)"))
		
		http.Error(w, "Erro Interno do Servidor (Simulado)", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Serviço 2 concluiu a tarefa!\n"))
}

func main() {
	tp := initTracer()
	defer tp.Shutdown(context.Background())


	handler := http.HandlerFunc(tarefaPesadaHandler)
	wrappedHandler := otelhttp.NewHandler(handler, "Servico2-Receber-HTTP")

	http.Handle("/processar", wrappedHandler)

	fmt.Println("Serviço 2 a arrancar na porta 8081...")
	log.Fatal(http.ListenAndServe(":8081", nil))
}