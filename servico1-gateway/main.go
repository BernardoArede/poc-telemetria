package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time" 

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/go-chi/chi/v5"

	pb "poc-telemetria/ProtocolBuffers"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

var contadorPedidos metric.Int64Counter
var tracer trace.Tracer

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
	contadorPedidos.Add(r.Context(), 1)

	ctx, span := tracer.Start(r.Context(), "Receber Tarefa HTTP")
	defer span.End()

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		http.Error(w, "Erro ao ligar ao NATS", http.StatusInternalServerError)
		return
	}
	defer nc.Close()

	tarefa := &pb.Tarefa{
		IdTarefa:  fmt.Sprintf("JOB-%d", time.Now().Unix()), 
		TipoAcao:  "GERAR_RELATORIO",
		Payload:   "dados_do_utilizador=12345;formato=pdf",
		Timestamp: time.Now().Unix(),
	}

	dadosBinarios, err := proto.Marshal(tarefa)
	if err != nil {
		http.Error(w, "Erro a empacotar dados Protobuf", http.StatusInternalServerError)
		return
	}

	msg := &nats.Msg{
		Subject: "TAREFAS.processamento",
		Data:    dadosBinarios,
		Header:  make(nats.Header), 
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(msg.Header)))

	err = nc.PublishMsg(msg)
	if err != nil {
		http.Error(w, "Erro ao publicar no NATS", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Tarefa submetida com sucesso para processamento assincrono!"))
}

func main() {
	res, _ := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceName("servico1-gateway"),
	))

	tp, _ := initTracer(res)
	defer tp.Shutdown(context.Background())
	tracer = tp.Tracer("servico1-gateway-tracer")

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

	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/processarEntrada", processHandler)
	})

	fmt.Println("Micro-Serviço 1 (Gateway) a arrancar na porta 8000...")

	log.Fatal(http.ListenAndServe(":8000", r))
}