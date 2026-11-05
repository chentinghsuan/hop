package ui

import (
	"sync"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
)

func TestSelectTemplate_OnlyCallsDescriptionOnFocusedOption(t *testing.T) {
	var calledIndices []int
	desc := func(value string, index int) string {
		calledIndices = append(calledIndices, index)
		return ""
	}

	opts := []core.OptionAnswer{
		{Index: 0, Value: "first"},
		{Index: 1, Value: "second"},
		{Index: 2, Value: "third"},
	}

	tmplData := survey.SelectTemplateData{
		Select:        survey.Select{Message: "Pick one:", Options: []string{"first", "second", "third"}, Description: desc},
		PageEntries:   opts,
		SelectedIndex: 1, // Cursor is on "second" (index 1)
		Description:   desc,
		Config:        &survey.PromptConfig{},
	}

	_, _, err := core.RunTemplate(survey.SelectQuestionTemplate, tmplData)
	if err != nil {
		t.Fatalf("RunTemplate failed: %v", err)
	}

	if len(calledIndices) == 0 {
		t.Fatalf("expected Description to be called for focused index 1, got none")
	}
	for _, idx := range calledIndices {
		if idx != 1 {
			t.Fatalf("expected Description to be called ONLY for focused index 1, got %v", calledIndices)
		}
	}
}

func TestHoverHandler_DebounceRapidCursorMoves(t *testing.T) {
	var (
		mu      sync.Mutex
		hovered []int
	)

	descFn, stop := newHoverHandler(func(index int) {
		mu.Lock()
		hovered = append(hovered, index)
		mu.Unlock()
	})
	defer stop()

	// Simulate rapid cursor movement: 0 -> 1 -> 2 (within 20ms each, less than 100ms debounce)
	descFn("opt0", 0)
	time.Sleep(20 * time.Millisecond)
	descFn("opt1", 1)
	time.Sleep(20 * time.Millisecond)
	descFn("opt2", 2)

	// Wait for debounce (100ms + margin)
	time.Sleep(140 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(hovered) != 1 || hovered[0] != 2 {
		t.Fatalf("expected only index 2 to be prefetched after rapid scrolling, got %v", hovered)
	}
}
