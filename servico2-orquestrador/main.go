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
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"google.golang.org/protobuf/proto"

	"github.com/nats-io/nats.go"

	telemetria "altice-openTelemetry"
	pb "poc-telemetria/ProtocolBuffers"
)

func main() {
	tp, lp, err := telemetria.InitConfig("servico2-worker", "localhost:4317")
	if err != nil {
		log.Fatal("Erro a iniciar telemetria:", err)
	}
	log.Println("[Orquestrador] Telemetria inicializada com sucesso")
	defer tp.Shutdown(context.Background())
	defer lp.Shutdown(context.Background())

	tracer := otel.Tracer("servico2-worker-tracer")
	logger := otelslog.NewLogger("worker-logger")

	log.Println("[Orquestrador] A ligar ao NATS em nats://localhost:4222")
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("Erro a ligar ao NATS:", err)
	}

	log.Println("[Orquestrador] Ligação ao NATS estabelecida com sucesso")
	defer nc.Close()

	logger.Info("Worker Iniciado e à escuta de Tarefas Genéricas...")
	log.Println("[Orquestrador] Worker iniciado e à escuta no subject TAREFAS.processamento")
	log.Println("[Orquestrador] Queue group utilizado: grupo-workers")

	_, err = telemetria.SubscribeWithTrace(nc, "TAREFAS.processamento", "grupo-workers", func(ctx context.Context, msg *nats.Msg) {

	nomeDoWorker := fmt.Sprintf("Worker-PID-%d", os.Getpid())

	log.Printf("[Orquestrador] Mensagem recebida do NATS | subject=%s | worker=%s", msg.Subject, nomeDoWorker)

	ctx, span := tracer.Start(ctx, "Executar Tarefa (Worker)")
	defer span.End()

	log.Println("[Orquestrador] Span iniciado: Executar Tarefa (Worker)")

	span.SetAttributes(attribute.String("worker.id", nomeDoWorker))

	var tarefa pb.Tarefa
	err := proto.Unmarshal(msg.Data, &tarefa)
	if err != nil {
		log.Println("[Orquestrador] Erro ao desserializar mensagem Protobuf:", err)
		logger.Error("Erro a ler o Protobuf", "erro", err.Error())
		return
	}

	log.Printf("[Orquestrador] Tarefa desserializada com sucesso: id=%s | acao=%s", tarefa.IdTarefa, tarefa.TipoAcao)

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

	log.Printf("[Orquestrador] A processar tarefa: id=%s | payload=%s", tarefa.IdTarefa, tarefa.Payload)

	time.Sleep(2 * time.Second)

	logger.Info("Tarefa processada com sucesso!", slog.String("id_tarefa", tarefa.IdTarefa))
	log.Printf("[Orquestrador] Tarefa processada com sucesso: id=%s", tarefa.IdTarefa)

	log.Println("[Orquestrador] A preparar chamada HTTP para o Serviço 3 - Auditoria")

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://localhost:8001/api/v1/auditoria", nil)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	log.Println("[Orquestrador] Contexto de tracing injetado nos headers HTTP")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("[Orquestrador] Erro ao contactar o Serviço 3 - Auditoria:", err)
		logger.Error("Erro a contactar o Serviço Final de Auditoria", "erro", err.Error())
	} else {
		defer resp.Body.Close()

		log.Printf("[Orquestrador] Serviço de Auditoria respondeu com status: %s", resp.Status)

		logger.Info("Serviço de Auditoria confirmou a gravação da Tarefa!")
		log.Println("[Orquestrador] Fluxo funcional concluído com sucesso")
	}
})

	select {}
}