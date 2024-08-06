package server

import (
	"fmt"
	"strings"
)

type FTPCommand struct {
	name     string
	argument string
}

var helpMenuHolder = map[string]string{
	"CD":   "------Currently in development------",
	"PUT":  "------Currently in development------",
	"GET":  "------Currently in development------",
	"CWD":  "------Currently in development------",
	"QUIT": "Disconnect client connection with server.",
	"HELP": "------Currently in development------",
}

func parseCommand(str string) FTPCommand {
	cmd := strings.SplitN(strings.TrimSpace(str), " ", 2)
	named := FTPCommand{name: strings.ToUpper(cmd[0])}
	if len(cmd) > 1 {
		named.argument = cmd[1]
	}

	return named
}

func (S *FTPServer) handleClientCommand(c *FTPClient, cmd FTPCommand) error {
	fmt.Println(cmd.name, cmd.argument)
	switch cmd.name {
	case "HELP":
		return c.handleHelpCommand()
	case "QUIT":
		return S.closeClientConnection(c)
	default:
		return nil
	}
}
