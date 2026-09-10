package managed

import "testing"

func TestStatePreconditions(t *testing.T) {
	for _, tc := range []struct {
		action  Action
		state   string
		execute bool
	}{
		{ActionStart, "stopped", true}, {ActionStart, "running", false}, {ActionShutdown, "running", true}, {ActionShutdown, "stopped", false}, {ActionStop, "running", true}, {ActionReboot, "running", true}, {ActionReboot, "stopped", false},
	} {
		got := CheckState(tc.action, tc.state, 3052)
		if got.Execute != tc.execute || (!tc.execute && got.Message == "") {
			t.Fatalf("%s/%s=%+v", tc.action, tc.state, got)
		}
	}
}
