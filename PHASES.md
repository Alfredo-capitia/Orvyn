# Orvyn - Roadmap por Fases

Este documento serve como guia para manter o fio lógico do projeto. Sempre que avançarmos para uma nova fase, marcamos as tarefas aqui.

**Regras:**
- Seguir rigorosamente fase por fase
- Só avançar após testes passarem
- Read-only sempre que possível
- Segurança por design
- Plan-first, least privilege, local-first

---

## Fase 0: Fundação & Git [EM PROGRESSO]
**Objetivo:** Base sólida, Git inicializado, documentação inicial

### Tarefas
- [x] Inicializar repositório Git
- [x] Criar `.gitignore`
- [ ] Criar `README.md`
- [ ] Criar `SECURITY.md`
- [ ] Verificar estrutura e dependências
- [ ] Compilar com sucesso

### Critérios de aceite
- `go build ./...` sem erros
- `.gitignore` cobre binários, state, backups
- Documentação básica presente

### Status: IN PROGRESS

---

## Fase 1: Core de Segurança
**Objetivo:** Consolidar módulos de segurança reutilizáveis

### Tarefas
- [x] `internal/fsutil/` - SafeJoin, path traversal protection
- [x] `internal/redaction/` - Redação de segredos
- [x] `internal/audit/` - Logging de eventos sem segredos
- [ ] `internal/state/` - Gestão de estado com permissões restritas
- [ ] `internal/config/` - Configuração com Viper
- [ ] Testes unitários para fsutil, redaction, audit

### Critérios de aceite
- Bloqueia path traversal
- Redact sempre ativo
- State com permissões 0600
- Testes passam

### Status: PENDING

---

## Fase 2: Docker Wrapper (read-only)
**Objetivo:** Abstração segura sobre Docker SDK

### Tarefas
- [ ] `internal/docker/client.go` - Cliente com timeouts
- [ ] `Ping()`, `Info()`, `ListContainers()`
- [ ] `ContainerLogs()` preparado para streaming
- [ ] Tratamento de erros seguro

### Critérios de aceite
- Usa apenas Docker SDK
- Sem shell commands
- Operações read-only
- Funciona com Docker socket

### Status: PENDING

---

## Fase 3: Discovery (read-only)
**Objetivo:** Descobrir ambiente sem alterações

### Tarefas
- [ ] `internal/discovery/discover.go`
- [ ] Detectar SO, Docker, containers
- [ ] Detectar projetos Docker Compose
- [ ] Detectar stack existente (Prometheus/Grafana/Loki/Alloy/cadvisor/node-exporter)
- [ ] Integrar com `orvyn discover`

### Critérios de aceite
- Completamente read-only
- Não lê .env
- Identifica stack existente corretamente
- Output claro e legível

### Status: PENDING

---

## Fase 4: Logs no Terminal
**Objetivo:** Feature diferenciadora `orvyn logs`

### Tarefas
- [ ] `internal/logs/stream.go` - Streaming via Docker API
- [ ] Integração com redaction
- [ ] Flags: `--follow`, `--tail`, `--timestamps`
- [ ] Resolução de serviço/container
- [ ] Escape seguro de terminal sequences
- [ ] Integrar com `orvyn logs`

### Critérios de aceite
- Usa Docker API
- Redaction sempre ativa
- Funciona em tempo real
- Sem persistência por defeito
- Ctrl+C funciona corretamente

### Status: PENDING

---

## Fase 5: Plan Engine
**Objetivo:** Gerar plano sem executar

### Tarefas
- [ ] `internal/planner/plan.go`
- [ ] Comparar discovered vs desired
- [ ] Gerar ações detalhadas
- [ ] Identificar backups necessários
- [ ] Integrar com `orvyn plan`

### Critérios de aceite
- 100% read-only
- Plano claro e completo
- Mostra impactos
- Legível

### Status: PENDING

---

## Fase 6: Stack Generation & Apply
**Objetivo:** Criar stack isolada com máxima segurança

### Tarefas
- [ ] `internal/stack/generate.go` - Gerar docker-compose para stack mínima
- [ ] Stack isolada em `/opt/orvyn/stack/`
- [ ] Confirmação interativa obrigatória
- [ ] Backups automáticos
- [ ] Criar diretórios com permissões corretas
- [ ] Aplicar via docker compose
- [ ] Audit log
- [ ] Integrar com `orvyn apply`

### Critérios de aceite
- Nunca mexe em docker-compose existente
- Confirmação obrigatória (só 'y')
- Backup antes de alterações
- Audit sem segredos
- Idempotente

### Status: PENDING

---

## Fase 7: Validate & Status
**Objetivo:** Validação e visibilidade

### Tarefas
- [ ] `internal/validate/checks.go`
- [ ] Verificar containers UP
- [ ] Verificar targets Prometheus
- [ ] Verificar Loki
- [ ] Integrar `orvyn validate` e `orvyn status`
- [ ] Mensagens com ações sugeridas

### Critérios de aceite
- Healthchecks funcionais
- Erros sem segredos
- Sugestões práticas

### Status: PENDING

---

## Log de Progresso
| Fase | Data Início | Data Fim | Status | Notas
|---|---|---|---|---|
| Fase 0 | 2025-10-24 | - | IN PROGRESS | Git iniciado, estrutura criada
