package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

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
	// 1. O OpenTelemetry cria um Span (Início do Trace)
	ctx, span := tracer.Start(r.Context(), "Receber Alarme HTTP")
	defer span.End()

	// 2. Conectar ao NATS (Na vida real isto faz-se no main, mas aqui é para a PoC)
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		http.Error(w, "Erro ao ligar ao NATS", http.StatusInternalServerError)
		return
	}
	defer nc.Close()

	// 3. Criar a nossa mensagem Protobuf (O Alarme da Altice)
	alarme := &pb.AlarmeRede{
		IdAlarme:        "ALM-999",
		EquipamentoIp:   "10.64.2.115",
		TipoEquipamento: "Antena-5G",
		NivelSeveridade: 5, // Crítico!
		DescricaoFalha:  "Perda de sinal ótico (LOS) devido a tempestade",
	}

	// 4. Compactar para Binário (Marshal)
	dadosBinarios, _ := proto.Marshal(alarme)

	// 5. Preparar a mensagem NATS
	msg := &nats.Msg{
		Subject: "ALARMES.rede",
		Data:    dadosBinarios,
		Header:  make(nats.Header), // Inicializar os cabeçalhos do NATS
	}

	// 6. O GOLPE DE MESTRE: Injetar o TraceID do OpenTelemetry nos Cabeçalhos do NATS
	// Usamos o propagador standard do Otel. Como o nats.Header é idêntico ao http.Header por baixo, fazemos um "cast"
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(http.Header(msg.Header)))

	// 7. Publicar no NATS
	err = nc.PublishMsg(msg)
	if err != nil {
		http.Error(w, "Erro ao publicar no NATS", http.StatusInternalServerError)
		return
	}

	// 8. Responder ao utilizador
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Alarme recebido e enviado para processamento assíncrono!"))
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

	// Vantagem do Chi: Podemos agrupar rotas de forma limpa.
	// E mais tarde podemos adicionar middlewares aqui (ex: r.Use(MeuMiddlewareOtel))
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/processarEntrada", processHandler)
	})

	fmt.Println("Micro-Serviço 1 (com go-chi) a arrancar na porta 8000...")

	log.Fatal(http.ListenAndServe(":8000", r))
}
