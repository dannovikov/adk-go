// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adka2a

import (
	"fmt"
	"slices"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"google.golang.org/genai"

	"google.golang.org/adk/session"
)

type inputRequiredProcessor struct {
	reqCtx *a2asrv.RequestContext
	event  *a2a.TaskStatusUpdateEvent
}

func newInputRequiredProcessor(reqCtx *a2asrv.RequestContext) *inputRequiredProcessor {
	return &inputRequiredProcessor{reqCtx: reqCtx}
}

func (p *inputRequiredProcessor) process(event *session.Event) error {
	resp := event.LLMResponse
	if resp.Content == nil {
		return nil
	}

	var inputRequiredParts []*genai.Part
	var longRunningCallIDs []string
	for _, part := range resp.Content.Parts {
		if part.FunctionCall != nil && slices.Contains(event.LongRunningToolIDs, part.FunctionCall.ID) {
			inputRequiredParts = append(inputRequiredParts, part)
			longRunningCallIDs = append(longRunningCallIDs, part.FunctionCall.ID)
			continue
		}
		if part.FunctionResponse != nil && p.isResponseToLongRunning(part.FunctionResponse.ID) {
			inputRequiredParts = append(inputRequiredParts, part)
			longRunningCallIDs = append(longRunningCallIDs, part.FunctionResponse.ID)
		}
	}

	if len(inputRequiredParts) == 0 {
		return nil
	}

	a2aParts, err := ToA2AParts(inputRequiredParts, longRunningCallIDs)
	if err != nil {
		return fmt.Errorf("failed to convert input required parts to A2A parts: %w", err)
	}

	if p.event != nil {
		p.event.Status.Message.Parts = append(p.event.Status.Message.Parts, a2aParts...)
		return nil
	}

	msg := a2a.NewMessage(a2a.MessageRoleAgent, a2aParts...)
	ev := a2a.NewStatusUpdateEvent(p.reqCtx, a2a.TaskStateInputRequired, msg)
	ev.Final = true
	p.event = ev
	return nil
}

func (p *inputRequiredProcessor) isResponseToLongRunning(id string) bool {
	if p.event == nil {
		return false
	}
	for _, part := range p.event.Status.Message.Parts {
		if dp, ok := part.(a2a.DataPart); ok {
			if typeVal, ok := dp.Metadata[a2aDataPartMetaTypeKey]; ok && typeVal == a2aDataPartTypeFunctionCall {
				if callID, ok := dp.Data["id"].(string); ok && callID == id {
					return true
				}
			}
		}
	}
	return false
}

// validateInputRequiredResumption checks if the input message contains responses to all function calls
// that happened during the previous invocation and were recorded in the Task input-required state message.
func validateInputRequiredResumption(reqCtx *a2asrv.RequestContext, content *genai.Content) error {
	if reqCtx.StoredTask == nil {
		return nil
	}
	task, statusMsg := reqCtx.StoredTask, reqCtx.StoredTask.Status.Message
	if task.Status.State != a2a.TaskStateInputRequired || statusMsg == nil {
		return nil
	}

	taskParts, err := ToGenAIParts(statusMsg.Parts)
	if err != nil {
		return fmt.Errorf("failed to parse task status message: %w", err)
	}

	for _, p := range taskParts {
		if p.FunctionCall == nil {
			continue
		}
		hasMatchingResponse := slices.ContainsFunc(content.Parts, func(p *genai.Part) bool {
			return p.FunctionResponse != nil && p.FunctionResponse.ID == p.FunctionCall.ID
		})
		if !hasMatchingResponse {
			return fmt.Errorf("no input provided for function call ID %q", p.FunctionCall.ID)
		}
	}
	return nil
}
