package main

import "testing"

func TestNudgeText(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"stuck?", true},
		{"Are you stuck", true},
		{"still at it?", true},
		{"Still running?", true},
		{"still doing it?", true},
		{"still going?", true},
		{"how's it going?", true},
		{"How’s it going", true},
		{"how is it going", true},
		{"are you there?", true},
		{"Are you alive?", true},
		{"continue", true},
		{"Continue.", true},
		{"ok continue", true},
		{"yes, continue!", true},
		{"ok then continue", true},
		{"keep going", true},
		{"go on", true},
		{"try now", true},
		{"Try again please", true},
		{"retry", true},
		{"ok retry it", true},
		{"done?", true},
		{"are you done?", true},
		{"status?", true},
		{"any update?", true},
		{"Any progress on this?", true},
		// Known limit: a short request that mentions a status word counts.
		{"I got stuck on the form, can you fill it in for me please", true},
		{"  continue  ", true},
		// "continue" and "retry" count only in messages of at most 3 words.
		{"continue with the migration to staging and then deploy", false},
		{"ok so please continue", false},
		{"retry the deploy with the new token", false},
		{"continued", false},
		{"done", false},
		{"status report for the board", false},
		{"what is the status of the release", false},
		{"Add the retry flag.", false},
		{"", false},
		// Over 60 characters after trimming never counts.
		{"are you stuck? the build has been running for a long while now", false},
	} {
		if got := nudgeText(tc.text); got != tc.want {
			t.Errorf("nudgeText(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestCloseOffText(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"close-off", true},
		{"/close-off", true},
		{"Please Close Off the session", true},
		{"ok, close-off now and clean up", true},
		{"close the offline cache", false},
		{"close offline mode", false},
		{"closeoff", false},
		{"Add the retry flag.", false},
	} {
		if got := closeOffText(tc.text); got != tc.want {
			t.Errorf("closeOffText(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestPer100(t *testing.T) {
	if rate := per100(1, 0); rate != nil {
		t.Errorf("per100 with no follow-ups = %v, want nil", *rate)
	}
	if rate := per100(1, 3); rate == nil || *rate != 33.3 {
		t.Errorf("per100(1, 3) = %v, want 33.3", rate)
	}
	if text := formatRate(nil); text != "n/a" {
		t.Errorf("formatRate(nil) = %q", text)
	}
	rate := 50.0
	if text := formatRate(&rate); text != "50.0" {
		t.Errorf("formatRate(50) = %q", text)
	}
}
