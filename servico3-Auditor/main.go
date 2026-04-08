package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

// Variável global para o nosso Tracer
var tracer trace.Tracer

// initTracer liga este serviço ao Otel Collector no Docker
func initTracer() (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Configurar o exportador GRPC para falar com o Collector na porta 4317
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint("localhost:4317"))
	if err != nil {
		return nil, err
	}

	// Definir o nome do serviço para aparecer bonitinho no HyperDX
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("Serviço 4 - Auditoria Altice"),
	))
	if err != nil {
		return nil, err
	}

	// Criar o Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Definir o propagador standard (Isto é OBRIGATÓRIO para extrair os IDs dos cabeçalhos!)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(tp)

	return tp, nil
}

func main() {
	// 1. Inicializar a Observabilidade
	tp, err := initTracer()
	if err != nil {
		log.Fatal("Erro a inicializar o Tracer:", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatal("Erro a desligar o Tracer:", err)
		}
	}()

	// Dar nome ao Tracer
	tracer = tp.Tracer("auditoria-tracer")

	// 2. Definir a rota HTTP que os Workers vão chamar
	http.HandleFunc("/api/v1/auditoria", func(w http.ResponseWriter, r *http.Request) {
		
		// 🌟 A MAGIA ACONTECE AQUI 🌟
		// Vamos aos cabeçalhos HTTP (r.Header) e extraímos o TraceID que o Worker Alpha/Beta enviou
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Iniciamos o Span passando esse "ctx" (contexto). O HyperDX vai ligar os pontos automaticamente!
		ctx, span := tracer.Start(ctx, "Gravar Resolução do Alarme (Serviço 4)")
		defer span.End()

		// Simulamos o trabalho deste serviço (escrever numa base de dados central)
		fmt.Println("🗄️ [AUDITORIA] Recebi pedido do Worker! A gravar resolução no sistema central...")
		time.Sleep(500 * time.Millisecond) // Simula atraso da Base de Dados
		fmt.Println("✅ [AUDITORIA] Ticket fechado com sucesso na Base de Dados!")

		// Responder ao Worker que correu tudo bem
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Auditoria registada com sucesso"))
	})

	// 3. Arrancar o Servidor HTTP
	fmt.Println("Serviço 4 (Auditoria) a correr na porta 8001...")
	log.Fatal(http.ListenAndServe(":8001", nil))
}