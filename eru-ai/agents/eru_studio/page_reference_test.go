package eru_studio

import "testing"

func ledgerWith(request string, pages []PageRef) *Ledger {
	ledger := NewLedger()
	ledger.RecordRequest(request)
	ledger.RecordListedPages(pages)
	return ledger
}

func TestAPageTheRequestNamesAndNobodyReadIsReported(t *testing.T) {
	ledger := ledgerWith(
		"match the grid on this page with the grid on invoice_360_detail",
		[]PageRef{{PageId: "p_inv", Name: "invoice_360_detail"}, {PageId: "p_other", Name: "financier_form"}},
	)
	unread := ledger.UnreadReferencedPages("p_current")
	if len(unread) != 1 || unread[0] != "invoice_360_detail" {
		t.Fatalf("got %v", unread)
	}
}

func TestAPageThatWasActuallyReadIsNotReported(t *testing.T) {
	ledger := ledgerWith(
		"copy the header from invoice_360_detail",
		[]PageRef{{PageId: "p_inv", Name: "invoice_360_detail"}},
	)
	ledger.RecordFetchedPage("p_inv")
	if unread := ledger.UnreadReferencedPages("p_current"); len(unread) != 0 {
		t.Fatalf("a page that was read was still faulted: %v", unread)
	}
}

func TestThePageBeingEditedIsNotAReference(t *testing.T) {
	ledger := ledgerWith(
		"tidy up the filters on er_dashboard",
		[]PageRef{{PageId: "p_current", Name: "er_dashboard"}},
	)
	if unread := ledger.UnreadReferencedPages("p_current"); len(unread) != 0 {
		t.Fatalf("the page under edit was treated as a reference: %v", unread)
	}
}

func TestAPageNobodyMentionedIsNotRequired(t *testing.T) {
	ledger := ledgerWith(
		"add a total row to the grid",
		[]PageRef{{PageId: "p_inv", Name: "invoice_360_detail"}},
	)
	if unread := ledger.UnreadReferencedPages("p_current"); len(unread) != 0 {
		t.Fatalf("an unmentioned page was required: %v", unread)
	}
}

func TestANameInsideAnotherNameDoesNotCount(t *testing.T) {
	// "invoice_360" must not match on the strength of "invoice_360_detail"
	// appearing in the sentence.
	ledger := ledgerWith(
		"make it like invoice_360_detail",
		[]PageRef{{PageId: "p_short", Name: "invoice_360"}, {PageId: "p_long", Name: "invoice_360_detail"}},
	)
	unread := ledger.UnreadReferencedPages("p_current")
	if len(unread) != 1 || unread[0] != "invoice_360_detail" {
		t.Fatalf("got %v", unread)
	}
}

func TestShortGenericNamesAreLeftAlone(t *testing.T) {
	ledger := ledgerWith(
		"put the form back at the top",
		[]PageRef{{PageId: "p_form", Name: "form"}},
	)
	if unread := ledger.UnreadReferencedPages("p_current"); len(unread) != 0 {
		t.Fatalf("a generic name turned an ordinary sentence into a lookup: %v", unread)
	}
}

func TestNothingIsRequiredWhenThePageListWasNeverRead(t *testing.T) {
	ledger := NewLedger()
	ledger.RecordRequest("make it like invoice_360_detail")
	if unread := ledger.UnreadReferencedPages("p_current"); len(unread) != 0 {
		t.Fatalf("got %v without a page list", unread)
	}
}
