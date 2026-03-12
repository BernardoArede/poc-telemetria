package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"go.opentelemetry.io/otel/codes"
)


var contadorPedidos metric.Int64Counter

func initTracer(res *resource.Resource) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:4318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res), 
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	
	return tp, nil
}

func initLogger(res *resource.Resource) (*sdklog.LoggerProvider, error) {
	ctx := context.Background()
	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint("localhost:4318"), 
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)
	return lp, nil
}

func initMetrics(res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(provider)
	return provider, nil
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("servico1-gateway")
	ctx, span := tracer.Start(r.Context(), "Receber-Pedido-Gateway")
	defer span.End()

	contadorPedidos.Add(ctx, 1)

	logger := otelslog.NewLogger("servico1-gateway")
	logger.InfoContext(ctx, "A preparar chamada para o Serviço 2...")

	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:8081/processar", nil)
	resp, err := client.Do(req)
	
	if err != nil || resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, "O Orquestrador falhou aleatoriamente")

		logger.ErrorContext(ctx, "Alarme! O Serviço 2 falhou durante o processamento.", "status_code", resp.StatusCode)
		
		http.Error(w, "Ocorreu um erro a processar o teu pedido", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	logger.InfoContext(ctx, "Chamada ao Serviço 2 concluída com sucesso!")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Pedido completo: Gateway -> Orquestrador\n"))
}


func main() {
	res, _ := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceName("servico1-gateway"),
	))

	tp, _ := initTracer(res)
	defer tp.Shutdown(context.Background())

	meterProvider, _ := initMetrics(res)
	
	meter := meterProvider.Meter("servico1-gateway-meter")
	contadorPedidos, _ = meter.Int64Counter("pedidos_total")

	lp, _ := initLogger(res)
	defer lp.Shutdown(context.Background())

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2222", mux)
	}()

	http.HandleFunc("/processarEntrada", processHandler)

	fmt.Println("Micro-Serviço 1 a arrancar na porta 8000...")
	log.Fatal(http.ListenAndServe(":8000", nil))
}