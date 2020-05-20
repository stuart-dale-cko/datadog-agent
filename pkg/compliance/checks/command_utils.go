// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

package checks

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

var (
	commandRunnerFunc func(context.Context, []string, bool) (int, []byte, error) = runCommand
)

func getDefaultShell() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"powershell", "-Command"}
	default:
		return []string{"sh", "-c"}
	}
}

func runCommand(ctx context.Context, command []string, captureStdout bool) (int, []byte, error) {
	if len(command) == 0 {
		return 0, nil, fmt.Errorf("Cannot run empty command")
	}

	_, err := exec.LookPath(command[0])
	if err != nil {
		return 0, nil, fmt.Errorf("Command '%s' not found, err: %v", command[0], err)
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if cmd == nil {
		return 0, nil, fmt.Errorf("Unable to create command context")
	}

	var stdoutBuffer bytes.Buffer
	if captureStdout {
		cmd.Stdout = &stdoutBuffer
	}

	err = cmd.Run()
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode(), stdoutBuffer.Bytes(), err
	}
	return -1, nil, fmt.Errorf("Unable to retrieve exit code, err: %v", err)
}
