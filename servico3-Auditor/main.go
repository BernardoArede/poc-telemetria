package main

import (
	"context"
	"fmt"
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
	defer tp.Shutdown(context.Background())
	defer lp.Shutdown(context.Background())

	tracer := otel.Tracer("servico3-auditoria-tracer")
	logger := otelslog.NewLogger("auditoria-logger")

	http.HandleFunc("/api/v1/auditoria", func(w http.ResponseWriter, r *http.Request) {
		
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		_, span := tracer.Start(ctx, "Gravar Registo na Base de Dados")
		defer span.End()

		logger.Info("🗄️ Pedido de auditoria recebido! A gravar resolução no sistema central...")
		
		time.Sleep(500 * time.Millisecond) 
		
		logger.Info("✅ Tarefa fechada com sucesso na Base de Dados!")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Auditoria registada com sucesso"))
	})

	fmt.Println("Serviço 3 (Auditoria) a correr na porta 8001...")
	log.Fatal(http.ListenAndServe(":8001", nil))
}