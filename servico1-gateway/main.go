package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	telemetria "altice-openTelemetry"
	pb "poc-telemetria/ProtocolBuffers"
)

var contadorPedidos metric.Int64Counter

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

	tracer := otel.Tracer("servico1-gateway-tracer")
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

	err = telemetria.PublishWithTrace(ctx, nc, "TAREFAS.processamento", dadosBinarios)
	if err != nil {
		http.Error(w, "Erro ao publicar no NATS", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Tarefa submetida com sucesso para processamento assincrono!"))
}

func main() {
	tp, lp, err := telemetria.InitConfig("servico1-gateway", "localhost:4317")
	if err != nil {
		log.Fatal("Erro a iniciar telemetria:", err)
	}
	defer tp.Shutdown(context.Background())
	defer lp.Shutdown(context.Background())
	
	res, _ := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceName("servico1-gateway"),
	))
	meterProvider, _ := initMetrics(res)
	meter := meterProvider.Meter("servico1-gateway-meter")
	contadorPedidos, _ = meter.Int64Counter("pedidos_total")

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