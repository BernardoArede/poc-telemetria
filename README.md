# POC Telemetria — Estágio na Altice Labs

> **Proof of Concept** de Observabilidade e Telemetria em Microsserviços, desenvolvido no âmbito do estágio curricular na **Altice Labs**.

---

## 📋 Índice

- [Sobre o Projeto](#sobre-o-projeto)
- [Arquitetura](#arquitetura)
- [Stack Tecnológica](#stack-tecnológica)
- [Estrutura do Repositório](#estrutura-do-repositório)
- [Pré-requisitos](#pré-requisitos)
- [Como Executar](#como-executar)
- [Serviços e Endpoints](#serviços-e-endpoints)
- [Telemetria](#telemetria)
- [Aceder às Ferramentas de Observabilidade](#aceder-às-ferramentas-de-observabilidade)

---

## Sobre o Projeto

Este repositório contém uma **Prova de Conceito (POC)** que demonstra a implementação de **observabilidade completa** numa arquitetura de microsserviços, utilizando o **OpenTelemetry** como standard de instrumentação.

O projeto foi desenvolvido durante o estágio curricular na [**Altice Labs**](https://www.alticelabs.com/), com o objetivo de estudar e validar estratégias modernas de telemetria distribuída, abrangendo os três pilares da observabilidade:

| Pilar | Ferramenta | Descrição |
|-------|-----------|-----------|
| 📊 **Métricas** | Prometheus + Grafana | Contadores e indicadores de desempenho |
| 🔍 **Rastreamento Distribuído** | Jaeger | Fluxo de pedidos entre serviços |
| 📝 **Logs Estruturados** | Loki + Grafana | Registos contextualizados com trace IDs |

O sistema simula ainda **falhas aleatórias** (taxa de falha de 30% no Serviço 2), permitindo observar o comportamento das ferramentas de telemetria perante erros reais.

---

## Arquitetura

### Fluxo de Pedidos

```
Cliente
  │
  ▼
┌──────────────────────────────────┐
│  Serviço 1 – Gateway             │
│  Porta: 8000 (/processarEntrada) │
│  Métricas: 2222 (/metrics)       │
│                                  │
│  • Recebe o pedido               │
│  • Incrementa contador           │
│  • Propaga contexto de trace     │
│  • Chama o Serviço 2             │
└────────────────┬─────────────────┘
                 │
                 ▼
┌──────────────────────────────────┐
│  Serviço 2 – Orquestrador        │
│  Porta: 8081 (/processar)        │
│                                  │
│  • Simula processamento (200ms)  │
│  • Injecta falhas aleatórias     │
│    (30% de probabilidade)        │
│  • Regista erros nos traces      │
└──────────────────────────────────┘
```

### Stack de Observabilidade

```
Serviço 1 ──────────────────────────────────────────────┐
Serviço 2 ──────────────────────────────────────────────┤
                                                         ▼
                                        ┌─────────────────────────────┐
                                        │   OpenTelemetry Collector   │
                                        │   OTLP HTTP  :4318          │
                                        │   OTLP gRPC  :4317          │
                                        └──────┬──────────────┬───────┘
                                               │              │
                              ┌────────────────┘              └──────────────┐
                              ▼                                              ▼
                  ┌───────────────────┐                        ┌────────────────────┐
                  │      Jaeger       │                        │        Loki        │
                  │   Traces :16686   │                        │   Logs  :3100      │
                  └───────────────────┘                        └────────┬───────────┘
                                                                        │
                                                              ┌─────────▼──────────┐
                  ┌───────────────────┐                       │                    │
                  │    Prometheus     │──────────────────────▶│      Grafana       │
                  │  Métricas :9090   │                       │  Dashboards :3000  │
                  └───────────────────┘                       └────────────────────┘
```

---

## Stack Tecnológica

### Microsserviços
| Tecnologia | Versão | Uso |
|-----------|--------|-----|
| **Go (Golang)** | 1.24+ | Linguagem dos serviços |
| `net/http` | stdlib | Servidor e cliente HTTP |

### OpenTelemetry (Instrumentação)
| Pacote | Finalidade |
|--------|-----------|
| `go.opentelemetry.io/otel` | SDK principal |
| `otelhttp` | Instrumentação automática HTTP |
| `otlptracehttp` | Exportação de traces via OTLP |
| `otlploghttp` | Exportação de logs via OTLP |
| `otelslog` | Bridge entre `slog` e OpenTelemetry |
| `prometheus` | Exportação de métricas |

### Observabilidade (Docker)
| Ferramenta | Versão | Função |
|-----------|--------|--------|
| **OpenTelemetry Collector** | latest | Recebe e encaminha telemetria |
| **Jaeger** | latest | Backend de rastreamento distribuído |
| **Prometheus** | latest | Coleta e armazenamento de métricas |
| **Grafana** | latest | Visualização e dashboards |
| **Loki** | latest | Agregação de logs |

---

## Estrutura do Repositório

```
poc-telemetria/
├── servico1-gateway/              # Serviço 1 — Gateway de entrada
│   ├── main.go                    # Lógica principal + instrumentação OTEL
│   ├── go.mod                     # Dependências Go
│   └── go.sum
│
├── servico2-orquestrador/         # Serviço 2 — Orquestrador com falhas caóticas
│   ├── main.go                    # Lógica principal + instrumentação OTEL
│   ├── go.mod
│   └── go.sum
│
├── otel-collector-config.yaml     # Configuração do OpenTelemetry Collector
├── prometheus.yml                 # Configuração do Prometheus
├── docker-compose.yml             # Orquestração de toda a stack de observabilidade
│
└── Reports/
    └── AlticeLabs_1ªReunião.pdf   # Relatório da 1ª reunião de estágio
```

---

## Pré-requisitos

- [Go](https://go.dev/dl/) 1.24 ou superior
- [Docker](https://docs.docker.com/get-docker/) e [Docker Compose](https://docs.docker.com/compose/install/)

---

## Como Executar

### 1. Iniciar a Stack de Observabilidade

Inicia o Jaeger, Prometheus, Grafana, Loki e o OpenTelemetry Collector:

```bash
docker-compose up -d
```

### 2. Iniciar o Serviço 2 — Orquestrador

```bash
cd servico2-orquestrador
go run main.go
```

O serviço fica disponível em `http://localhost:8081`.

### 3. Iniciar o Serviço 1 — Gateway

```bash
cd servico1-gateway
go run main.go
```

O serviço fica disponível em `http://localhost:8000`.

### 4. Enviar Pedidos de Teste

```bash
curl http://localhost:8000/processarEntrada
```

Pode repetir o comando várias vezes para observar o comportamento nos dashboards — incluindo as falhas aleatórias geradas pelo Serviço 2.

---

## Serviços e Endpoints

| Serviço | URL | Descrição |
|---------|-----|-----------|
| Gateway | `http://localhost:8000/processarEntrada` | Endpoint principal de entrada |
| Gateway (Métricas) | `http://localhost:2222/metrics` | Métricas Prometheus do Serviço 1 |
| Orquestrador | `http://localhost:8081/processar` | Endpoint interno do Serviço 2 |

---

## Telemetria

### Sinais Implementados

| Serviço | Traces | Métricas | Logs |
|---------|--------|---------|------|
| Serviço 1 – Gateway | ✅ Span `Receber-Pedido-Gateway` | ✅ Counter `pedidos_total` | ✅ Logs estruturados via `otelslog` |
| Serviço 2 – Orquestrador | ✅ Span `Processar-Logica` (com erro em falhas) | ❌ | ❌ |

### Propagação de Contexto

O Serviço 1 propaga automaticamente o **contexto de trace** nas chamadas HTTP ao Serviço 2 através da instrumentação `otelhttp`, criando spans pai-filho e permitindo visualizar o fluxo completo de um pedido no Jaeger.

### Injeção de Falhas (Chaos Engineering)

O Serviço 2 simula:
- ⏱️ **Latência**: delay artificial de **200ms** por pedido
- 💥 **Erros aleatórios**: **30%** de probabilidade de retornar `HTTP 500`

Isto permite validar como as ferramentas de observabilidade detetam e representam falhas em produção.

---

## Aceder às Ferramentas de Observabilidade

Após executar `docker-compose up -d`:

| Ferramenta | URL | Credenciais |
|-----------|-----|-------------|
| **Grafana** | http://localhost:3000 | `admin` / `admin` |
| **Jaeger** | http://localhost:16686 | — |
| **Prometheus** | http://localhost:9090 | — |

### Jaeger — Ver Traces

1. Acede a http://localhost:16686
2. Em *Service*, seleciona `servico1-gateway`
3. Clica em **Find Traces**
4. Explora o fluxo completo de cada pedido, incluindo os spans do Serviço 2

### Grafana — Ver Métricas e Logs

1. Acede a http://localhost:3000 (login: `admin`/`admin`)
2. Para **métricas**: adiciona Prometheus como datasource (`http://prometheus:9090`) e explora a métrica `pedidos_total`
3. Para **logs**: adiciona Loki como datasource (`http://loki:3100`) e pesquisa pelos logs do gateway

---

## Sobre o Estágio

Este projeto foi desenvolvido na **Altice Labs** como parte de um estágio curricular focado em tecnologias de observabilidade e monitorização de sistemas distribuídos.

**Altice Labs** é o centro de I&D do Grupo Altice em Portugal, sediado em Aveiro, e é referência nacional em investigação aplicada nas áreas de telecomunicações e tecnologias de informação.
