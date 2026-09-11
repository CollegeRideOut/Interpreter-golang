package main

import "testing"

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name            string
		arguments       []string
		wantInteractive bool
		wantDirectory   string
		wantError       bool
	}{
		{name: "default", wantDirectory: "TestProgram"},
		{name: "interactive", arguments: []string{"--interactive"}, wantInteractive: true, wantDirectory: "TestProgram"},
		{name: "interactive directory", arguments: []string{"--interactive", "./repo"}, wantInteractive: true, wantDirectory: "./repo"},
		{name: "directory", arguments: []string{"./repo"}, wantDirectory: "./repo"},
		{name: "unknown option", arguments: []string{"--wat"}, wantError: true},
		{name: "two directories", arguments: []string{"one", "two"}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			interactive, directory, err := parseArguments(test.arguments)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, want error = %t", err, test.wantError)
			}
			if err != nil {
				return
			}
			if interactive != test.wantInteractive || directory != test.wantDirectory {
				t.Fatalf("got interactive=%t directory=%q, want interactive=%t directory=%q", interactive, directory, test.wantInteractive, test.wantDirectory)
			}
		})
	}
}
