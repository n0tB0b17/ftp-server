package server

import (
	"strings"
)

type FTPCommand struct {
	name     string
	argument string
}

func parseCommand(str string) FTPCommand {
	cmd := strings.SplitN(strings.TrimSpace(str), " ", 2)
	named := FTPCommand{name: strings.ToUpper(cmd[0])}
	if len(cmd) > 1 {
		named.argument = cmd[1]
	}

	return named
}
