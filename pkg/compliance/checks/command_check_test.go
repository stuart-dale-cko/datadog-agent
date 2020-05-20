// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.
package checks

import (
	"context"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/compliance"
	"github.com/stretchr/testify/assert"
)

type commandFixture struct {
	test            *testing.T
	name            string
	check           commandCheck
	commandExitCode int
	commandOutput   string
	commandError    error
	expCommandInput []string
	expKV           compliance.KV
	expError        error
}

func (f *commandFixture) mockRunCommand(ctx context.Context, command []string, captureStdout bool) (int, []byte, error) {
	assert.ElementsMatch(f.test, f.expCommandInput, command)
	return f.commandExitCode, []byte(f.commandOutput), f.commandError
}

func (f *commandFixture) run(t *testing.T) {
	t.Helper()

	f.test = t
	reporter := f.check.reporter.(*compliance.MockReporter)
	commandRunnerFunc = f.mockRunCommand

	expectedCalls := 0
	if f.expKV != nil {
		reporter.On(
			"Report",
			newTestRuleEvent(
				nil,
				f.expKV,
			),
		).Once()
		expectedCalls = 1
	}

	err := f.check.Run()
	reporter.AssertNumberOfCalls(t, "Report", expectedCalls)
	assert.Equal(t, f.expError, err)
}

func newFakeCommandCheck(t *testing.T, command *compliance.Command) commandCheck {
	check, err := newCommandCheck(newTestBaseCheck(&compliance.MockReporter{}), command)
	assert.NoError(t, err)
	return *check
}

func TestCommandCheck(t *testing.T) {
	tests := []commandFixture{
		{
			name: "Simple case",
			check: newFakeCommandCheck(t, &compliance.Command{
				Run: "my command --foo=bar --baz",
				Report: compliance.Report{
					{
						As: "myCommandOutput",
					},
				},
			}),
			commandExitCode: 0,
			commandOutput:   "output",
			commandError:    nil,
			expCommandInput: append(getDefaultShell(), "my command --foo=bar --baz"),
			expKV: compliance.KV{
				"myCommandOutput": "output",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
