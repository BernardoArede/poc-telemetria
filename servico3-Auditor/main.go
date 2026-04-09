package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log/global"
)

func initTelemetry() (*sdktrace.TracerProvider, *sdklog.LoggerProvider, error) {
	ctx := context.Background()

	res, _ := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("servico3-auditoria"), 
	))


	traceExporter, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint("localhost:4317"))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logExporter, _ := otlploggrpc.New(ctx, otlploggrpc.WithInsecure(), otlploggrpc.WithEndpoint("localhost:4317"))
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)), sdklog.WithResource(res))
	global.SetLoggerProvider(lp)

	return tp, lp, nil
}

func main() {
	tp, lp, err := initTelemetry()
	if err != nil {
		log.Fatal("Erro a iniciar telemetria:", err)
	}
	defer tp.Shutdown(context.Background())
	defer lp.Shutdown(context.Background())

	tracer := otel.Tracer("servico3-auditoria-tracer")
	logger := otelslog.NewLogger("auditoria-logger")

	http.HandleFunc("/api/v1/auditoria", func(w http.ResponseWriter, r *http.Request) {
		
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		_, span := tracer.Start(ctx, "Gravar Registo na Base de Dados")
		defer span.End()

		logger.Info("🗄️ Pedido de auditoria recebido! A gravar resolução no sistema central...")
		
		time.Sleep(500 * time.Millisecond) // Simula atraso da Base de Dados
		
		logger.Info("✅ Tarefa fechada com sucesso na Base de Dados!")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Auditoria registada com sucesso"))
	})

	fmt.Println("Serviço 3 (Auditoria) a correr na porta 8001...")
	log.Fatal(http.ListenAndServe(":8001", nil))
}