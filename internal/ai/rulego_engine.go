package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"aigent/internal/database"
	"aigent/internal/utils"

	"github.com/mitchellh/mapstructure"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"gorm.io/gorm"
)

// AigentToolNodeConfiguration describe la configuración del nodo
type AigentToolNodeConfiguration struct {
	ToolName string `json:"toolName"`
}

// AigentToolNode es un nodo de RuleGo que ejecuta una herramienta de AIgent
type AigentToolNode struct {
	Config AigentToolNodeConfiguration
}

// Ensure it implements types.Node
var _ types.Node = (*AigentToolNode)(nil)

func (n *AigentToolNode) Type() string {
	return "aigent/tool"
}

func (n *AigentToolNode) New() types.Node {
	return &AigentToolNode{
		Config: AigentToolNodeConfiguration{},
	}
}

func (n *AigentToolNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return mapstructure.Decode(configuration, &n.Config)
}

func (n *AigentToolNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 1. Obtener la herramienta desde el Registry global
	// Necesitamos que el Brain se instancie de forma global o podamos acceder a él.
	// Para evitar acoplamientos complejos, accedemos al Registry global a través de GetGlobalBrain()
	brain := GetGlobalBrain()
	if brain == nil {
		ctx.TellNext(msg, types.Failure)
		return
	}

	tDef, exists := brain.Registry.GetBySanitized(n.Config.ToolName)
	if !exists {
		// Intentar buscar por nombre directo si no está sanitizado
		tDef, exists = brain.Registry.Get(n.Config.ToolName)
		if !exists {
			ctx.TellNext(msg, types.Failure)
			return
		}
	}

	// 2. Parsear argumentos desde msg.Data
	var args map[string]interface{}
	if msg.Data != nil && !msg.Data.IsEmpty() {
		_ = json.Unmarshal([]byte(msg.Data.Get()), &args)
	}
	if args == nil {
		args = make(map[string]interface{})
	}

	// 3. Ejecutar la herramienta en el contexto actual
	result, err := tDef.Execute(ctx.GetContext(), args)
	if err != nil {
		if msg.Data != nil {
			msg.Data.Set(fmt.Sprintf(`{"error": "%s"}`, err.Error()))
		}
		ctx.TellNext(msg, types.Failure)
		return
	}

	// 4. Pasar resultado exitoso
	if msg.Data != nil {
		msg.Data.Set(string(result))
	}
	ctx.TellNext(msg, types.Success)
}

func (n *AigentToolNode) Destroy() {
}

// GlobalBrainReference para acceder al cerebro desde los nodos de RuleGo
var (
	globalBrain *Brain
	brainMu     sync.RWMutex
)

func SetGlobalBrain(b *Brain) {
	brainMu.Lock()
	defer brainMu.Unlock()
	globalBrain = b
}

func GetGlobalBrain() *Brain {
	brainMu.RLock()
	defer brainMu.RUnlock()
	return globalBrain
}

// RuleGoConfig y orquestador de flujos
var (
	RuleGoConfig types.Config
	engineInit   sync.Once
)

// InitRuleGo inicializa el motor de RuleGo y registra nodos personalizados
func InitRuleGo() {
	engineInit.Do(func() {
		// Configurar callback de Debug para registrar telemetría y logs de ejecución
		RuleGoConfig = rulego.NewConfig(types.WithOnDebug(func(chainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			// Intentar extraer run_id de los metadatos
			if msg.Metadata == nil {
				return
			}
			runIDStr := msg.Metadata.GetValue("run_id")
			if runIDStr == "" {
				return
			}
			runID, _ := strconv.Atoi(runIDStr)
			if runID == 0 {
				return
			}

			// Escribir log en la base de datos
			var logLine string
			if err != nil {
				logLine = fmt.Sprintf("[%s] Node: %s -> relation: %s -> Error: %v\n", time.Now().Format("15:04:05"), nodeId, relationType, err)
			} else {
				var dataLen int
				if msg.Data != nil {
					dataLen = msg.Data.Len()
				}
				logLine = fmt.Sprintf("[%s] Node: %s -> relation: %s -> Output Length: %d\n", time.Now().Format("15:04:05"), nodeId, relationType, dataLen)
			}

			// Actualizar base de datos de forma segura (atómica)
			go func() {
				updates := map[string]interface{}{
					"current_node_id": nodeId,
					"logs":            gorm.Expr("logs || ?", logLine),
				}
				if relationType == types.Failure && err != nil {
					updates["status"] = "FAILED"
				}
				database.DB.Model(&database.WorkflowRun{}).Where("id = ?", runID).Updates(updates)
			}()
		}))

		// Registrar el nodo personalizado
		rulego.Registry.Register(&AigentToolNode{})
		log.Println("🔌 RuleGo: Custom nodes registered successfully.")
	})
}

// ReloadWorkflows lee todos los flujos activos de la base de datos y los carga en RuleGo
func ReloadWorkflows() error {
	InitRuleGo()

	var wfs []database.Workflow
	if err := database.DB.Where("enabled = ?", true).Find(&wfs).Error; err != nil {
		return err
	}

	for _, wf := range wfs {
		// Normalizar y validar definición JSON
		normalized, err := utils.NormalizeRuleChainJSON(wf.Definition)
		if err != nil {
			log.Printf("⚠️ RuleGo: Workflow '%s' has invalid or unnormalizable JSON: %v", wf.Name, err)
			continue
		}

		// Registrar o actualizar la RuleChain
		_, err = rulego.New(strconv.Itoa(int(wf.ID)), []byte(normalized), rulego.WithConfig(RuleGoConfig))
		if err != nil {
			log.Printf("⚠️ RuleGo: Failed to initialize RuleChain for workflow '%s': %v", wf.Name, err)
			continue
		}
		log.Printf("✅ RuleGo: Loaded Workflow '%s' (ID #%d) into engine", wf.Name, wf.ID)
	}

	return nil
}

// TriggerWorkflow arranca una ejecución asíncrona de un flujo RuleGo
func TriggerWorkflow(ctx context.Context, workflowID uint, payload string) (uint, error) {
	InitRuleGo()

	var wf database.Workflow
	if err := database.DB.First(&wf, workflowID).Error; err != nil {
		return 0, fmt.Errorf("workflow not found: %w", err)
	}

	// 1. Crear el registro de la ejecución (Run)
	run := database.WorkflowRun{
		WorkflowID: workflowID,
		Status:     "RUNNING",
		Logs:       fmt.Sprintf("[%s] Starting Workflow run...\n", time.Now().Format("15:04:05")),
	}
	if err := database.DB.Create(&run).Error; err != nil {
		return 0, err
	}

	// 2. Obtener el motor de la RuleChain cargada
	engine, exists := rulego.Get(strconv.Itoa(int(workflowID)))
	if !exists {
		// Intentar cargar al vuelo si no estaba registrado
		normalized, err := utils.NormalizeRuleChainJSON(wf.Definition)
		if err != nil {
			run.Status = "FAILED"
			run.Logs += fmt.Sprintf("[%s] Error normalizing rule chain: %v\n", time.Now().Format("15:04:05"), err)
			database.DB.Save(&run)
			return run.ID, err
		}

		engine, err = rulego.New(strconv.Itoa(int(workflowID)), []byte(normalized), rulego.WithConfig(RuleGoConfig))
		if err != nil {
			run.Status = "FAILED"
			run.Logs += fmt.Sprintf("[%s] Error loading rule chain: %v\n", time.Now().Format("15:04:05"), err)
			database.DB.Save(&run)
			return run.ID, err
		}
	}

	// 3. Crear el mensaje para RuleGo y setear run_id en metadatos
	meta := types.NewMetadata()
	meta.PutValue("run_id", strconv.Itoa(int(run.ID)))
	msg := types.NewMsg(0, "AIGENT_TRIGGER", types.JSON, meta, payload)

	// 4. Ejecutar de forma asíncrona
	go func() {
		// Envolver ejecución con recuperación en caso de pánico
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️ RuleGo: Panic during workflow run #%d: %v", run.ID, r)
				panicLog := fmt.Sprintf("[%s] Panic recovered: %v\n", time.Now().Format("15:04:05"), r)
				database.DB.Model(&database.WorkflowRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
					"status": "FAILED",
					"logs":   gorm.Expr("logs || ?", panicLog),
				})
			}
		}()

		// Lanzar
		engine.OnMsg(msg, types.WithContext(context.Background()), types.WithDebugMode(true), types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
			var finalStatus = "COMPLETED"
			var logLine = fmt.Sprintf("[%s] Workflow finished execution.\n", time.Now().Format("15:04:05"))
			if err != nil {
				finalStatus = "FAILED"
				logLine += fmt.Sprintf("[%s] Error: %v\n", time.Now().Format("15:04:05"), err)
			}

			// Actualizar estado final de forma atómica para evitar colisiones de concurrencia
			database.DB.Model(&database.WorkflowRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
				"status": finalStatus,
				"logs":   gorm.Expr("logs || ?", logLine),
			})
		}))
	}()

	return run.ID, nil
}
