package main

import (
	"chat/globals"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCountInputTokensFromRecordedPromptsUsesFixedImageEstimate(t *testing.T) {
	image := "data:image/png;base64," + strings.Repeat("A", 4096)
	withImage := mustPromptJSON(t, "gpt-image-2", []globals.Message{{
		Role:    globals.User,
		Content: "describe this image " + image,
	}})
	withoutImage := mustPromptJSON(t, "gpt-image-2", []globals.Message{{
		Role:    globals.User,
		Content: "describe this image ",
	}})

	withTokens, err := countInputTokensFromRecordedPrompts("gpt-image-2", withImage)
	if err != nil {
		t.Fatalf("count with image: %v", err)
	}

	withoutTokens, err := countInputTokensFromRecordedPrompts("gpt-image-2", withoutImage)
	if err != nil {
		t.Fatalf("count without image: %v", err)
	}

	if delta := withTokens - withoutTokens; delta != 3000 {
		t.Fatalf("image token delta = %d, want 3000", delta)
	}
}

func TestRecomputeRecordRepairsInflatedTotalTokens(t *testing.T) {
	prompts := mustPromptJSON(t, "gpt-image-2-all", []globals.Message{{
		Role:    globals.User,
		Content: "please edit this " + "data:image/png;base64," + strings.Repeat("A", 4096),
	}})
	record := modelCallRecord{
		ID:           1285,
		Model:        "gpt-image-2-all",
		InputTokens:  6000,
		OutputTokens: 50,
		TotalTokens:  5550723,
		Input:        prompts,
		Output:       "![image](/api/attachments/example.png)",
		CreatedAt:    time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}

	repair, err := recomputeRecord(record)
	if err != nil {
		t.Fatalf("recompute record: %v", err)
	}
	if repair == nil {
		t.Fatal("expected repair")
	}
	if repair.NewInputTokens >= repair.OldInputTokens {
		t.Fatalf("new input tokens = %d, want less than old input tokens %d", repair.NewInputTokens, repair.OldInputTokens)
	}
	if repair.NewTotalTokens != repair.NewInputTokens+repair.NewOutputTokens {
		t.Fatalf("new total = %d, want %d", repair.NewTotalTokens, repair.NewInputTokens+repair.NewOutputTokens)
	}
	if repair.OldTotalTokens == repair.NewTotalTokens {
		t.Fatal("expected total tokens to change")
	}
}

func TestSummarizeRepairsGroupsRedisDeltas(t *testing.T) {
	repairs := []repairResult{
		{ID: 1, Day: "2026-05-31", Model: "gpt-image-2", OldTotalTokens: 100, NewTotalTokens: 10},
		{ID: 2, Day: "2026-05-31", Model: "gpt-image-2", OldTotalTokens: 200, NewTotalTokens: 20},
		{ID: 3, Day: "2026-06-01", Model: "gpt-image-2-all", OldTotalTokens: 50, NewTotalTokens: 30},
	}

	summaries := summarizeRepairs(repairs)
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}

	if summaries[0].Day != "2026-05-31" || summaries[0].Model != "gpt-image-2" || summaries[0].Rows != 2 || summaries[0].Delta != -270 {
		t.Fatalf("unexpected first summary: %+v", summaries[0])
	}
	if summaries[1].Day != "2026-06-01" || summaries[1].Model != "gpt-image-2-all" || summaries[1].Rows != 1 || summaries[1].Delta != -20 {
		t.Fatalf("unexpected second summary: %+v", summaries[1])
	}
}

func mustPromptJSON(t *testing.T, model string, messages []globals.Message) string {
	t.Helper()

	data, err := json.Marshal(recordedPrompts{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		t.Fatalf("marshal prompts: %v", err)
	}

	return string(data)
}
