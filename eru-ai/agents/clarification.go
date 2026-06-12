package agents

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ClarificationAnswersParamKey = "clarification_answers"
	ResumeContextParamKey        = "resume_context"
)

// QuestionAction returns the first action on the message whose ActionType is
// "question", if any.
func (m AgentMessage) QuestionAction() (AgentOutputAction, bool) {
	for _, a := range m.Actions {
		if a.ActionType == ActionTypeQuestion {
			return a, true
		}
	}
	return AgentOutputAction{}, false
}

// ClarificationAnswers extracts structured answers sent back by the UI via
// AgentMessage.Params[ClarificationAnswersParamKey].
func (m AgentMessage) ClarificationAnswers() ([]ClarificationAnswer, bool) {
	if m.Params == nil {
		return nil, false
	}
	raw, ok := m.Params[ClarificationAnswersParamKey]
	if !ok {
		return nil, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var answers []ClarificationAnswer
	if err := json.Unmarshal(b, &answers); err != nil {
		return nil, false
	}
	if len(answers) == 0 {
		return nil, false
	}
	return answers, true
}

// PendingQuestion scans loaded history (most recent first) and returns the last
// assistant message that asked a clarification question. Callers use this to
// detect that an incoming message is an answer to a pending question.
func PendingQuestion(conversation *Conversation) (AgentMessage, AgentOutputAction, bool) {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		msg := conversation.Messages[i]
		if action, ok := msg.QuestionAction(); ok {
			return msg, action, true
		}
		if msg.Role == "user" {
			break
		}
	}
	return AgentMessage{}, AgentOutputAction{}, false
}

// ParseClarificationRequest converts the action payload of a question action
// back into a typed ClarificationRequest.
func ParseClarificationRequest(action map[string]interface{}) (ClarificationRequest, error) {
	var req ClarificationRequest
	b, err := json.Marshal(action)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return req, err
	}
	return req, nil
}

// ToAction builds the question AgentOutputAction carrying this request.
func (req ClarificationRequest) ToAction(agentName string) AgentOutputAction {
	action := map[string]interface{}{}
	b, _ := json.Marshal(req)
	_ = json.Unmarshal(b, &action)
	return AgentOutputAction{
		ActionType: ActionTypeQuestion,
		ActionName: agentName,
		Action:     action,
	}
}

// FormatAnswersForModel renders the asked questions and the user's answers into
// a single text block so the model sees, on resume, what it asked and what was
// chosen. When the original request is empty (questions not available) it falls
// back to rendering the answers alone.

func FormatAnswersForModel(req ClarificationRequest, answers []ClarificationAnswer) string {
	questionById := make(map[string]ClarificationQuestion, len(req.Questions))
	for _, q := range req.Questions {
		questionById[q.Id] = q
	}

	var sb strings.Builder
	sb.WriteString("The user answered the clarification questions:\n")
	for _, ans := range answers {
		if q, ok := questionById[ans.QuestionId]; ok {
			sb.WriteString(fmt.Sprintf("- Q: %s\n", q.Question))
		} else {
			sb.WriteString(fmt.Sprintf("- Q (%s):\n", ans.QuestionId))
		}
		var parts []string
		if len(ans.Selected) > 0 {
			parts = append(parts, strings.Join(ans.Selected, ", "))
		}
		if strings.TrimSpace(ans.FreeText) != "" {
			parts = append(parts, ans.FreeText)
		}
		answerText := strings.Join(parts, " | ")
		if answerText == "" {
			answerText = "(no answer)"
		}
		sb.WriteString(fmt.Sprintf("  A: %s\n", answerText))
	}
	return strings.TrimRight(sb.String(), "\n")
}
