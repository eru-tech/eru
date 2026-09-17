package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	models "github.com/eru-tech/eru/eru-ai/models"
)

func encodedImage(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestThePlannerSeesAPictureItCanAfford(t *testing.T) {
	original := photographicImage(t, 1600, 1200)
	files := []models.FileMessage{{FileName: "screenshot.png", FileType: "image/png", ImageData: original}}

	out := planningAttachments(context.Background(), files)
	if len(out) != 1 || out[0].ImageData == "" {
		t.Fatal("the planner was left with no picture at all")
	}
	if len(out[0].ImageData) >= len(original) {
		t.Fatalf("the preview is not smaller: %d vs %d", len(out[0].ImageData), len(original))
	}

	raw, err := base64.StdEncoding.DecodeString(out[0].ImageData)
	if err != nil {
		t.Fatalf("preview is not base64: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("preview is not an image: %v", err)
	}
	if cfg.Width > planningPreviewPixels || cfg.Height > planningPreviewPixels {
		t.Fatalf("preview is %dx%d, larger than the %dpx limit", cfg.Width, cfg.Height, planningPreviewPixels)
	}
}

// photographicImage is noisy enough not to compress away to nothing, the way a
// real screenshot or photograph behaves.
func photographicImage(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(12345)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			img.Set(x, y, color.RGBA{uint8(seed >> 16), uint8(seed >> 8), uint8(seed), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestAPreviewThatWouldBeBiggerIsNotUsed(t *testing.T) {
	// A synthetic gradient compresses better as its original PNG than as a
	// downscaled JPEG. Sending the larger one would cost more and show less.
	original := encodedImage(t, 1600, 1200)
	out := planningAttachments(context.Background(), []models.FileMessage{{FileName: "gradient.png", ImageData: original}})
	if out[0].ImageData != original {
		t.Fatalf("swapped in a preview that was larger: %d vs %d", len(out[0].ImageData), len(original))
	}
}

func TestASmallImageIsLeftAsItIs(t *testing.T) {
	original := encodedImage(t, 64, 64)
	out := planningAttachments(context.Background(), []models.FileMessage{{FileName: "icon.png", ImageData: original}})
	if out[0].ImageData != original {
		t.Fatal("an image already within the preview size was re-encoded for no reason")
	}
}

func TestTheDocumentBodyIsNotSentToThePlanner(t *testing.T) {
	// The planner routes a document; it does not read one.
	out := planningAttachments(context.Background(), []models.FileMessage{
		{FileName: "terms.pdf", FileType: "application/pdf", FileData: "AAAA"},
	})
	if out[0].FileData != "" {
		t.Fatal("the document body was sent to the planner")
	}
	if out[0].FileName != "terms.pdf" {
		t.Fatal("the planner cannot tell what was attached")
	}
}

func TestSomethingThatIsNotAnImageIsPassedOnUnharmed(t *testing.T) {
	out := planningAttachments(context.Background(), []models.FileMessage{
		{FileName: "broken.png", FileType: "image/png", ImageData: "not-an-image"},
	})
	if out[0].ImageData != "not-an-image" {
		t.Fatal("an undecodable payload was silently discarded rather than passed on")
	}
}

func TestThePlannerIsToldWhatWasAttachedAndWhereItGoes(t *testing.T) {
	summary := attachmentSummary([]models.FileMessage{
		{FileName: "mockup.png", FileType: "image/png", FileId: "f-1"},
		{FileName: "terms.pdf", FileType: "application/pdf"},
	})
	for _, want := range []string{"user.files.0", "mockup.png", "f-1", "user.files.1", "terms.pdf", "top-level"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the attachment summary does not mention %q:\n%s", want, summary)
		}
	}
}

func TestNoSummaryWhenNothingWasAttached(t *testing.T) {
	if summary := attachmentSummary(nil); summary != "" {
		t.Fatalf("invented an attachment summary: %q", summary)
	}
}

func TestWithoutAFileStoreTheAttachmentStillTravels(t *testing.T) {
	// Losing the attachment would be worse than carrying it inline.
	oa := &OrchestratorAgent{}
	files := []models.FileMessage{{FileName: "a.png", ImageData: "AAAA"}}
	out := oa.offloadAttachments(context.Background(), files, "processo", "t1")
	if len(out) != 1 || out[0].ImageData != "AAAA" {
		t.Fatalf("the attachment was dropped when no store was configured: %+v", out)
	}
}

func TestTheStoredIdIsFoundWhateverTheProviderCallsIt(t *testing.T) {
	for _, shape := range []map[string]interface{}{
		{"file_id": "a"},
		{"id": "a"},
		{"doc_id": "a"},
		{"result": map[string]interface{}{"fileId": "a"}},
	} {
		if got := storedFileId(shape); got != "a" {
			t.Fatalf("id not found in %v, got %q", shape, got)
		}
	}
	if got := storedFileId(map[string]interface{}{"status": "ok"}); got != "" {
		t.Fatalf("invented an id: %q", got)
	}
}
