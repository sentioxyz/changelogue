package agent

import (
	"fmt"
	"strings"

	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/plugin/loggingplugin"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
)

// tracedAgentTool wraps a sub-agent as a tool (like agenttool.New) but attaches
// the logging plugin to the sub-agent's runner so that internal tool calls and
// LLM interactions are visible in the trace output.
type tracedAgentTool struct {
	agent agent.Agent
}

// newTracedAgentTool creates a new traced agent tool.
func newTracedAgentTool(a agent.Agent) tool.Tool {
	return &tracedAgentTool{agent: a}
}

func (t *tracedAgentTool) Name() string        { return t.agent.Name() }
func (t *tracedAgentTool) Description() string  { return t.agent.Description() }
func (t *tracedAgentTool) IsLongRunning() bool  { return false }

func (t *tracedAgentTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: &genai.Schema{
			Type: "OBJECT",
			Properties: map[string]*genai.Schema{
				"request": {Type: "STRING"},
			},
			Required: []string{"request"},
		},
	}
}

func (t *tracedAgentTool) Run(toolCtx tool.Context, args any) (map[string]any, error) {
	margs, ok := args.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("tracedAgentTool expects map[string]any arguments, got %T", args)
	}

	input, ok := margs["request"]
	if !ok {
		return nil, fmt.Errorf("missing required argument 'request' for agent %s", t.agent.Name())
	}
	inputText, ok := input.(string)
	if !ok {
		inputText = fmt.Sprint(input)
	}
	content := genai.NewContentFromText(inputText, genai.RoleUser)

	sessionService := session.InMemoryService()

	r, err := runner.New(runner.Config{
		AppName:        t.agent.Name(),
		Agent:          t.agent,
		SessionService: sessionService,
		PluginConfig: runner.PluginConfig{
			Plugins: []*plugin.Plugin{loggingplugin.MustNew("agent_trace")},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner for sub-agent %s: %w", t.agent.Name(), err)
	}

	stateMap := make(map[string]any)
	for k, v := range toolCtx.State().All() {
		if !strings.HasPrefix(k, "_adk") {
			stateMap[k] = v
		}
	}

	subSession, err := sessionService.Create(toolCtx, &session.CreateRequest{
		AppName: t.agent.Name(),
		UserID:  toolCtx.UserID(),
		State:   stateMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session for sub-agent %s: %w", t.agent.Name(), err)
	}

	eventCh := r.Run(toolCtx, subSession.Session.UserID(), subSession.Session.ID(), content, agent.RunConfig{})

	var lastEvent *session.Event
	for event, err := range eventCh {
		if err != nil {
			return nil, fmt.Errorf("error during execution of sub-agent %s: %w", t.agent.Name(), err)
		}
		if event.ErrorCode != "" || event.ErrorMessage != "" {
			return nil, fmt.Errorf("error from sub-agent %q (code: %q, message: %q)", t.agent.Name(), event.ErrorCode, event.ErrorMessage)
		}
		if event.LLMResponse.Content != nil {
			lastEvent = event
		}
	}

	if lastEvent == nil {
		return map[string]any{}, nil
	}

	lastContent := lastEvent.LLMResponse.Content
	var textParts []string
	for _, part := range lastContent.Parts {
		if part != nil && part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	outputText := strings.Join(textParts, "\n")
	if outputText == "" {
		return map[string]any{}, nil
	}
	return map[string]any{"result": outputText}, nil
}

func (t *tracedAgentTool) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}
	name := t.Name()
	if _, ok := req.Tools[name]; ok {
		return fmt.Errorf("duplicate tool: %q", name)
	}
	req.Tools[name] = t

	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	decl := t.Declaration()
	if decl == nil {
		return nil
	}
	var funcTool *genai.Tool
	for _, gt := range req.Config.Tools {
		if gt != nil && gt.FunctionDeclarations != nil {
			funcTool = gt
			break
		}
	}
	if funcTool == nil {
		req.Config.Tools = append(req.Config.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{decl},
		})
	} else {
		funcTool.FunctionDeclarations = append(funcTool.FunctionDeclarations, decl)
	}
	return nil
}
