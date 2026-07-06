package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/contrib/bridges/otelslog"


	telemetria "altice-openTelemetry"
)

func main() {
	tp, lp, err := telemetria.InitConfig("servico3-auditoria", "localhost:4317")
	if err != nil {
		log.Fatal("Erro a iniciar telemetria:", err)
	}
	log.Println("[Auditoria] Telemetria inicializada com sucesso")
	defer tp.Shutdown(context.Background())
	defer lp.Shutdown(context.Background())

	tracer := otel.Tracer("servico3-auditoria-tracer")
	logger := otelslog.NewLogger("auditoria-logger")

	http.HandleFunc("/api/v1/auditoria", func(w http.ResponseWriter, r *http.Request) {

		log.Println("[Auditoria] Pedido recebido em /api/v1/auditoria")

		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		log.Println("[Auditoria] Contexto de tracing extraído dos headers HTTP")

		_, span := tracer.Start(ctx, "Gravar Registo na Base de Dados")
		defer span.End()

		log.Println("[Auditoria] Span iniciado: Gravar Registo na Base de Dados")

		logger.Info("Pedido de auditoria recebido. A gravar resolução no sistema central...")
		log.Println("[Auditoria] A simular gravação da tarefa no sistema central")

		time.Sleep(500 * time.Millisecond)

		logger.Info("Tarefa fechada com sucesso na Base de Dados!")
		log.Println("[Auditoria] Tarefa fechada com sucesso")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Auditoria registada com sucesso"))

		log.Println("[Auditoria] Resposta enviada ao Orquestrador: 200 OK")
	})

	log.Println("[Auditoria] Serviço disponível em http://localhost:8001")
	log.Println("[Auditoria] Endpoint principal: POST /api/v1/auditoria")
	log.Fatal(http.ListenAndServe(":8001", nil))
}