package reasoning_agents

import (
	"context"
	"strings"
	"testing"

	agents "github.com/eru-tech/eru/eru-ai/agents"
	studio "github.com/eru-tech/eru/eru-ai/agents/eru_studio"
)

// The sub-agent runs under "<conversation>::<agent>". That is how a conversation
// is addressed; it was never a page id.
const theSubAgentConversation = "ce59af78-979b-4ca1-9c41-56a784d0369a::eru_studio"

func TestThePageBeingEditedKeepsItsOwnId(t *testing.T) {
	base := map[string]interface{}{"id": "business_card_capture", "name": "business_card_capture"}

	identity := resolvePageIdentity(nil, base, theSubAgentConversation)
	if identity.Id != "business_card_capture" || !identity.Fixed {
		t.Fatalf("identity = %+v, want the page's own id, fixed", identity)
	}

	// And the answer cannot rename it: a renamed page is saved as a second page
	// instead of updating the one the user is editing.
	answer := map[string]interface{}{"id": "something_the_model_preferred"}
	replaced, changed := identity.Stamp(answer)
	if !changed || replaced != "something_the_model_preferred" || answer["id"] != "business_card_capture" {
		t.Errorf("a renamed page was not put back: %v", answer)
	}
}

func TestAClientThatIsNotSendingThePageCanStillNameIt(t *testing.T) {
	params := map[string]interface{}{studio.PageIdParam: "invoice_detail"}
	identity := resolvePageIdentity(params, nil, theSubAgentConversation)
	if identity.Id != "invoice_detail" || !identity.Fixed {
		t.Fatalf("identity = %+v", identity)
	}

	// The page in code still wins over the param - it is the page in front of the user.
	base := map[string]interface{}{"id": "the_real_page"}
	if identity := resolvePageIdentity(params, base, theSubAgentConversation); identity.Id != "the_real_page" {
		t.Errorf("the param overrode the page being edited: %+v", identity)
	}

	// A blank or null param is not an id.
	for _, blank := range []interface{}{"", "   ", nil} {
		identity := resolvePageIdentity(map[string]interface{}{studio.PageIdParam: blank}, nil, "conv")
		if identity.Fixed {
			t.Errorf("%#v was taken as a page id", blank)
		}
	}
}

// The regression: the id handed to the model was the sub-agent's conversation
// id, suffix and all.
func TestANewPageIsNeverIdentifiedByTheConversationItWasBuiltIn(t *testing.T) {
	identity := resolvePageIdentity(nil, nil, theSubAgentConversation)

	if identity.Id == theSubAgentConversation {
		t.Error("the page id is the sub-agent's conversation id")
	}
	if strings.Contains(identity.Id, "::") {
		t.Errorf("the page id carries a conversation suffix: %q", identity.Id)
	}
	if identity.Fixed {
		t.Error("a page that does not exist yet was treated as having a fixed identity")
	}
	// Stable across the turns of one build, so a client that does not round-trip
	// the page does not get a new page every turn.
	if again := resolvePageIdentity(nil, nil, theSubAgentConversation); again.Id != identity.Id {
		t.Errorf("the fallback id is not stable: %q then %q", identity.Id, again.Id)
	}
	if identity.Id != "ce59af78-979b-4ca1-9c41-56a784d0369a" {
		t.Errorf("fallback = %q", identity.Id)
	}

	// With no conversation at all it is still a usable id.
	if bare := resolvePageIdentity(nil, nil, ""); bare.Id == "" {
		t.Error("no id at all")
	}
}

// The standoff: the request said one id, the prompt asked for another, and the
// model reasoned about which to obey instead of building the page.
func TestTheRequestDoesNotArgueWithThePromptAboutTheName(t *testing.T) {
	newPage := resolvePageIdentity(nil, nil, theSubAgentConversation)
	augment := buildEruStudioContextAugmentation(context.Background(), nil, newPage)
	if strings.Contains(augment, newPage.Id) {
		t.Errorf("a page that has no id yet was still assigned one in the prompt:\n%s", augment)
	}
	if strings.Contains(strings.ToLower(augment), "erupage.id") {
		t.Errorf("the prompt instructs on the id it has no business naming:\n%s", augment)
	}

	// So the name the user asked for is what the page gets.
	asked := map[string]interface{}{"id": "business_card_capture"}
	if _, changed := newPage.Stamp(asked); changed || asked["id"] != "business_card_capture" {
		t.Errorf("the id the user asked for was overwritten: %v", asked)
	}
	// An answer that named nothing still comes back identified.
	unnamed := map[string]interface{}{}
	if _, changed := newPage.Stamp(unnamed); !changed || unnamed["id"] != newPage.Id {
		t.Errorf("an answer with no id was left unidentified: %v", unnamed)
	}

	// When the page does exist, the prompt states its real id.
	existing := resolvePageIdentity(nil, map[string]interface{}{"id": "invoice_detail"}, theSubAgentConversation)
	stated := buildEruStudioContextAugmentation(context.Background(), nil, existing)
	if !strings.Contains(stated, "invoice_detail") {
		t.Errorf("the prompt does not state the id of the page being edited:\n%s", stated)
	}
	if strings.Contains(stated, theSubAgentConversation) {
		t.Errorf("the conversation id reached the prompt:\n%s", stated)
	}
}

func TestABarePageAnswerComesBackUnderTheRightId(t *testing.T) {
	ctx := studio.WithPageIdentity(context.Background(), studio.PageIdentity{Id: "invoice_detail", Fixed: true})
	output := agents.AgentMessage{Actions: []agents.AgentOutputAction{
		{ActionType: agents.ActionTypeAnswer, Action: map[string]interface{}{"id": "renamed_by_the_model", "name": "invoice"}},
		{ActionType: agents.ActionTypeQuestion, Action: map[string]interface{}{"questions": []interface{}{}}},
	}}

	stamped := eruStudioStampBarePage(ctx, output)
	if got := stamped.Actions[0].Action["id"]; got != "invoice_detail" {
		t.Errorf("page id = %v", got)
	}
	// A question is the agent asking, not answering - it carries no page.
	if _, touched := stamped.Actions[1].Action["id"]; touched {
		t.Error("a question action was stamped with a page id")
	}
}

// The clarification the user was shown offered fixed slots, one entry, or a note
// that nested pages would be needed - three ways to not get what was asked for,
// and no way to ask for it. Single-page mode is the only mode where the agent
// cannot resolve that itself, so the option that fixes it has to be offered.
func TestSinglePageModeOffersTheWayOutOfSinglePageMode(t *testing.T) {
	instructions := eruStudioModeInstructions(studio.ModeFull)
	if instructions == "" {
		t.Fatal("single-page mode carries no instructions")
	}
	if !strings.Contains(instructions, studio.OptionEnableMultiPage) {
		t.Errorf("single-page mode never tells the agent to offer the multi-page option:\n%s", instructions)
	}
	// It has to be the first option and the recommendation, or it reads as the
	// exotic choice among three sensible-looking compromises.
	for _, want := range []string{"first option", "AFTER it", "re-issue"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("the instruction does not establish %q:\n%s", want, instructions)
		}
	}

	// The modes that can carry a nested page have nothing to ask about.
	for _, mode := range []string{studio.ModeAuto, studio.ModePatch} {
		if strings.Contains(eruStudioModeInstructions(mode), studio.OptionEnableMultiPage) {
			t.Errorf("%s mode offers to enable what it already has", mode)
		}
	}
}

func TestTheMultiPageOptionValueIsStable(t *testing.T) {
	// The client keys off this string to re-issue the request with multi-page
	// output; renaming it silently breaks that handling.
	if studio.OptionEnableMultiPage != "enable_multi_page" {
		t.Errorf("reserved option value = %q", studio.OptionEnableMultiPage)
	}
}
