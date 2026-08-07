package mqtt

import "testing"

func TestOutputHasError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "server error", output: "getRole: Error: Role not found\n", want: true},
		{name: "case insensitive", output: "createClient: ERROR: Client exists\n", want: true},
		{name: "warning only", output: "Warning: connection is not encrypted\nadmin\n", want: false},
		{name: "empty", output: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := outputHasError([]byte(test.output)); got != test.want {
				t.Fatalf("outputHasError() = %v, want %v", got, test.want)
			}
		})
	}
}
