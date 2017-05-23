package console

import (
	"bufio"
	"os"
	"strings"
)

func ReadLine() (string, error) {
	text, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	text = strings.TrimRight(text, "\n\r")
	return text, nil
}

type Console struct {
	empty    func(string)
	commands []*Command
}

type Command struct {
	Name     string
	Callback func(string)
}

func NewConsole(empty func(string)) *Console {
	cons := &Console{
		commands: make([]*Command, 0),
		empty:    empty,
	}
	return cons
}

func (cons *Console) Command(name string, callback func(string)) {
	command := &Command{
		Name:     name,
		Callback: callback,
	}
	cons.commands = append(cons.commands, command)
}

func (cons *Console) Run() {
	for {
		text, err := ReadLine()
		if err != nil {
			panic("Error reading console input")
		}
		found := false
		for _, command := range cons.commands {
			prefix := command.Name + " "
			if strings.HasPrefix(text, prefix) {
				parts := strings.Split(text, " ")
				command.Callback(parts[1])
				found = true
				break
			}
		}
		if !found {
			cons.empty(text)
		}
	}
}
