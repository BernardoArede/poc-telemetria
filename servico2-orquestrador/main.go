package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/attribute"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/protobuf/proto"
	


	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log/global"

	"math/rand"

	"go.opentelemetry.io/otel/codes"

	pb "poc-telemetria/ProtocolBuffers"

	"github.com/nats-io/nats.go"

	
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

func initTelemetry() (*sdktrace.TracerProvider, *sdklog.LoggerProvider, error) {
	ctx := context.Background()

	// Identidade do Serviço (Aparece no HyperDX e no Grafana!)
	res, _ := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("Serviço 2 - Worker Altice"),
	))

	// 1. Configurar Traces
	traceExporter, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint("localhost:4317"))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

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
	defer lp.Shutdown(context.Background()) // Garantir que os logs são enviados antes de fechar

	tracer := otel.Tracer("servico2-orquestrador")
	
	// 🌟 NOVO: Criar o nosso Logger interligado com o OpenTelemetry
	logger := otelslog.NewLogger("worker-logger")

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("Erro a ligar ao NATS:", err)
	}
	defer nc.Close()

	// Em vez de Println, usamos o logger!
	logger.Info("Worker Alpha (Serviço 2) arrancou e está à escuta de Alarmes...")

	_, err = nc.QueueSubscribe("ALARMES.rede", "trabalhadores-altice", func(msg *nats.Msg) {
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(http.Header(msg.Header)))
		
		nomeDoWorker := os.Getenv("NOME_DO_WORKER")
		if nomeDoWorker == "" {
			nomeDoWorker = fmt.Sprintf("Worker-PID-%d", os.Getpid())
		}

		ctx, span := tracer.Start(ctx, "Processar Alarme (WORKER)")
		defer span.End()
		span.SetAttributes(attribute.String("altice.worker.id", nomeDoWorker))

		var alarme pb.AlarmeRede
		err := proto.Unmarshal(msg.Data, &alarme)
		if err != nil {
			logger.Error("Erro a ler o Protobuf", "erro", err.Error())
			return
		}

		span.SetAttributes(
			attribute.String("altice.alarme.id", alarme.IdAlarme),
			attribute.String("altice.equipamento", alarme.TipoEquipamento),
		)

		// 🌟 NOVO: Log Estruturado! Ele vai para o Grafana com as variáveis separadas
		logger.Info("Alarme Recebido", 
			slog.String("worker", nomeDoWorker),
			slog.String("id_alarme", alarme.IdAlarme),
			slog.String("equipamento", alarme.TipoEquipamento),
		)      
		
		time.Sleep(2 * time.Second)
		logger.Info("Alarme processado com sucesso!", slog.String("id_alarme", alarme.IdAlarme))

		req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:8001/api/v1/auditoria", nil)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Erro a contactar a Auditoria", "erro", err.Error())
		} else {
			resp.Body.Close()
			logger.Info("Auditoria confirmou a receção!")
		}
	})

	select {}
}