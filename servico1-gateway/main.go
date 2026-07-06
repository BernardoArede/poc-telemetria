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
	log.Println("[Gateway] Pedido recebido em /api/v1/processarEntrada")

	contadorPedidos.Add(r.Context(), 1)
	log.Println("[Gateway] Contador pedidos_total incrementado")

	tracer := otel.Tracer("servico1-gateway-tracer")
	ctx, span := tracer.Start(r.Context(), "Receber Tarefa HTTP")
	defer span.End()

	log.Println("[Gateway] Span iniciado: Receber Tarefa HTTP")

	log.Println("[Gateway] A ligar ao NATS em nats://localhost:4222")
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Println("[Gateway] Erro ao ligar ao NATS:", err)
		http.Error(w, "Erro ao ligar ao NATS", http.StatusInternalServerError)
		return
	}
	defer nc.Close()

	log.Println("[Gateway] Ligação ao NATS estabelecida com sucesso")

	tarefa := &pb.Tarefa{
		IdTarefa:  fmt.Sprintf("JOB-%d", time.Now().Unix()),
		TipoAcao:  "GERAR_RELATORIO",
		Payload:   "dados_do_utilizador=12345;formato=pdf",
		Timestamp: time.Now().Unix(),
	}

	log.Printf("[Gateway] Tarefa criada: id=%s | acao=%s", tarefa.IdTarefa, tarefa.TipoAcao)

	dadosBinarios, err := proto.Marshal(tarefa)
	if err != nil {
		log.Println("[Gateway] Erro ao serializar tarefa com Protobuf:", err)
		http.Error(w, "Erro a empacotar dados Protobuf", http.StatusInternalServerError)
		return
	}

	log.Printf("[Gateway] Tarefa serializada com Protocol Buffers: %d bytes", len(dadosBinarios))

	log.Println("[Gateway] A publicar tarefa no NATS no subject TAREFAS.processamento")
	err = telemetria.PublishWithTrace(ctx, nc, "TAREFAS.processamento", dadosBinarios)
	if err != nil {
		log.Println("[Gateway] Erro ao publicar tarefa no NATS:", err)
		http.Error(w, "Erro ao publicar no NATS", http.StatusInternalServerError)
		return
	}

	log.Printf("[Gateway] Tarefa publicada com sucesso no NATS: id=%s", tarefa.IdTarefa)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Tarefa submetida com sucesso para processamento assincrono!"))

	log.Println("[Gateway] Resposta enviada ao cliente: 202 Accepted")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	log.Println("[Gateway] A iniciar configuração de telemetria...")
	
	tp, lp, err := telemetria.InitConfig("servico1-gateway", "localhost:4317")
	if err != nil {
		log.Fatal("Erro a iniciar telemetria:", err)
	}
	
	log.Println("[Gateway] Telemetria inicializada com sucesso")
	
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

		log.Println("[Gateway] Endpoint de métricas disponível em http://localhost:2222/metrics")

		if err := http.ListenAndServe(":2222", mux); err != nil {
			log.Fatal("[Gateway] Erro ao iniciar servidor de métricas:", err)
		}
	}()

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/processarEntrada", processHandler)
	})

	log.Println("[Gateway] API HTTP disponível em http://localhost:8000")
	log.Println("[Gateway] Endpoint principal: POST /api/v1/processarEntrada")

	log.Fatal(http.ListenAndServe(":8000", r))
}