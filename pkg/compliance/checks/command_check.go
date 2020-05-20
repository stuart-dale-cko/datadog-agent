// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/pkg/compliance"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

const (
	defaultTimeoutSeconds  int = 30
	defaultOutputSizeLimit int = 10 * 1024
)

type commandCheck struct {
	baseCheck
	command        *compliance.Command
	shellCommand   []string
	commandTimeout time.Duration
	maxOutputSize  int
}

func newCommandCheck(baseCheck baseCheck, command *compliance.Command) (*commandCheck, error) {
	if len(command.Run) == 0 {
		return nil, fmt.Errorf("Unable to create commandCheck without a command to run")
	}

	// TODO: Find a way to put default values in `compliance.Command` object before reaching this function
	// As we should not modify this object here
	var shellCommand []string
	if len(command.ShellCommand) != 0 {
		shellCommand = command.ShellCommand
	} else {
		shellCommand = getDefaultShell()
	}
	shellCommand = append(shellCommand, command.Run)

	var timeout int
	if command.TimeoutSeconds != 0 {
		timeout = command.TimeoutSeconds
	} else {
		timeout = defaultTimeoutSeconds
	}

	var maxOutputSize int
	if command.MaxOutputSize != 0 {
		maxOutputSize = command.MaxOutputSize
	} else {
		maxOutputSize = defaultOutputSizeLimit
	}

	return &commandCheck{
		baseCheck:      baseCheck,
		command:        command,
		shellCommand:   shellCommand,
		commandTimeout: time.Duration(timeout) * time.Second,
		maxOutputSize:  maxOutputSize,
	}, nil
}

func (c *commandCheck) Run() error {
	log.Debugf("Command check: %v", c.command)

	context, cancel := context.WithTimeout(context.Background(), c.commandTimeout)
	defer cancel()

	// TODO: Capture stdout only when necessary
	exitCode, stdout, err := commandRunnerFunc(context, c.shellCommand, true)
	if exitCode == -1 && err != nil {
		return log.Warnf("Command '%v' execution failed - not reporting", c.command)
	}

	var shouldReport = false
	for _, filter := range c.command.Filter {
		if filter.Include != nil && filter.Include.ExitCode == exitCode {
			shouldReport = true
			break
		}
		if filter.Exclude != nil && filter.Exclude.ExitCode == exitCode {
			break
		}
	}

	// If we have no filtering we'll only accept 0
	if exitCode == 0 && !shouldReport {
		shouldReport = true
	}

	if shouldReport {
		c.reportCommand(stdout)
	} else {
		return log.Warnf("Command '%v' execution returned excluded exitcode: %d, error: %v", c.command, exitCode, err)
	}

	return nil
}

func (c *commandCheck) reportCommand(stdout []byte) error {
	if len(stdout) > c.maxOutputSize {
		return log.Errorf("Command '%v' output is too large: %d, won't be reported", c.command, len(stdout))
	}

	kv := compliance.KV{}
	strStdout := string(stdout)

	for _, field := range c.command.Report {
		if len(field.As) > 0 {
			kv[field.As] = strStdout
		}
	}

	if len(kv) > 0 {
		c.report(nil, kv)
	}

	return nil
}
