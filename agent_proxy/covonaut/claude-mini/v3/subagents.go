package main

import (
	"github.com/covoyage/covonaut/agentcore"
	"github.com/covoyage/covonaut/tools"
)

// buildTaskTool wires the "task" sub-agent meta-tool. The model calls it with
// an {agent, task} pair; the work runs in a fresh worker agent that only has
// read-only exploration tools, and the worker's final text is returned as the
// tool result. This keeps long investigations out of the main conversation —
// the same role v15's SubagentRunner + Task tool played.
func buildTaskTool(provider agentcore.Provider, model string) *agentcore.Tool {
	workerCfg := agentcore.Config{
		ModelConfig: agentcore.ModelConfig{
			Name:     "worker",
			Model:    model,
			Provider: provider,
		},
		SystemPrompt: workerPrompt,
		Tools:        workerTools(),
		ExecutionConfig: agentcore.ExecutionConfig{
			MaxTurns:          int64(maxTurnsFlag),
			ValidateArguments: true,
		},
	}
	if contextFlag > 0 {
		workerCfg.ContextWindow = contextFlag
	}

	worker := agentcore.TaskOption{
		Name:        "worker",
		Description: "Read-only code explorer. Give it a focused investigation (find files, grep, read sources) and it reports concise findings. Great for keeping the main context clean.",
		Tool:        agentcore.AgentAsTool(workerCfg),
	}
	return agentcore.TaskTool("task", []agentcore.TaskOption{worker})
}

// workerTools is the read-only tool set for the sub-agent: it can look around
// but never mutate the workspace, so a delegated investigation is safe.
func workerTools() []*agentcore.Tool {
	cwd := sandboxFlag
	return []*agentcore.Tool{
		tools.NewReadTool(cwd, nil),
		tools.NewGrepTool(cwd, nil),
		tools.NewGlobTool(cwd, nil),
		tools.NewFindTool(cwd, nil),
		tools.NewLsTool(cwd, nil),
	}
}

const workerPrompt = `You are a read-only research sub-agent. Explore the workspace
with read/grep/glob/find/ls to answer the delegated task precisely and concisely.
You cannot modify files or run shell commands. Report findings (file paths, line
references, and the relevant code), not actions.`
