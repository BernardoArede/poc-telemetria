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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/protobuf/proto"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log/global"

	pb "poc-telemetria/ProtocolBuffers"

	"github.com/nats-io/nats.go"
)

func initTelemetry() (*sdktrace.TracerProvider, *sdklog.LoggerProvider, error) {
	ctx := context.Background()

	res, _ := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("servico2-worker"), // Nome genérico
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

	tracer := otel.Tracer("servico2-worker-tracer")
	logger := otelslog.NewLogger("worker-logger")

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("Erro a ligar ao NATS:", err)
	}
	defer nc.Close()

	logger.Info("Worker Iniciado e à escuta de Tarefas Genéricas...")

	_, err = nc.QueueSubscribe("TAREFAS.processamento", "grupo-workers", func(msg *nats.Msg) {
		
		ctx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.HeaderCarrier(http.Header(msg.Header)))

		nomeDoWorker := fmt.Sprintf("Worker-PID-%d", os.Getpid())

		ctx, span := tracer.Start(ctx, "Executar Tarefa (Worker)")
		defer span.End()
		span.SetAttributes(attribute.String("worker.id", nomeDoWorker))
		var tarefa pb.Tarefa
		err := proto.Unmarshal(msg.Data, &tarefa)
		if err != nil {
			logger.Error("Erro a ler o Protobuf", "erro", err.Error())
			return
		}

		span.SetAttributes(
			attribute.String("tarefa.id", tarefa.IdTarefa),
			attribute.String("tarefa.acao", tarefa.TipoAcao),
		)

		logger.Info("Tarefa Recebida",
			slog.String("worker", nomeDoWorker),
			slog.String("id_tarefa", tarefa.IdTarefa),
			slog.String("acao", tarefa.TipoAcao),
			slog.String("payload", tarefa.Payload),
		)

		time.Sleep(2 * time.Second)
		logger.Info("Tarefa processada com sucesso!", slog.String("id_tarefa", tarefa.IdTarefa))
		req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:8001/api/v1/auditoria", nil)
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header)) // Injeta contexto HTTP

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Erro a contactar o Serviço Final de Auditoria", "erro", err.Error())
		} else {
			resp.Body.Close()
			logger.Info("Serviço de Auditoria confirmou a gravação da Tarefa!")
		}
	})

	select {}
}