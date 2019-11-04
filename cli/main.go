package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/chzyer/readline"

	"encoding/json"

	"github.com/otherview/protoactor-go/actor"
	"github.com/otherview/protoactor-go/remote"
	"github.com/gogo/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// Function constructor - constructs new function for listing given directory
var completer = readline.NewPrefixCompleter(
	readline.PcItem("tell"),
	readline.PcItem("watch"),
	readline.PcItem("exit"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

var echoPID *actor.PID
var rootContext = actor.EmptyRootContext

func main() {
	logo := `
     ___         _         ___ _    ___
    | _ \_ _ ___| |_ ___  / __| |  |_ _|
    |  _/ '_/ _ \  _/ _ \| (__| |__ | |
    |_| |_| \___/\__\___(_)___|____|___|
`
	fmt.Println(logo)

	remote.DefaultSerializerID = 1
	remote.Start("127.0.0.1:0")
	spawnEcho()

	vars := make(map[string]string)
	vars["%address%"] = actor.ProcessRegistry.Address
	vars["%echo%"] = fmt.Sprintf(`{"Address":"%v", "Id":"echo"}`, actor.ProcessRegistry.Address)

	l, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[31m»\033[0m ",
		HistoryFile:     "/tmp/readline.tmp",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})

	if err != nil {
		panic(err)
	}
	defer l.Close()

	log.SetOutput(l.Stderr())
	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		for k, v := range vars {
			line = strings.Replace(line, k, v, 1000)
		}
		switch {

		case strings.HasPrefix(line, "tell "):
			tell(line)
		case strings.HasPrefix(line, "watch "):
			watch(line)
		case line == "exit":
			goto exit
		case line == "":
		default:
			log.Println("Unknown command :", strconv.Quote(line))
		}
	}
exit:
}

func spawnEcho() {
	echoPID, _ = rootContext.SpawnNamed(actor.PropsFromFunc(func(ctx actor.Context) {
		switch msg := ctx.Message().(type) {
		case *actor.Started:
			fmt.Println("ECHO: Started")
		case *watchRequest:
			fmt.Printf("ECHO: Watching %v\n", msg.target.String())
			ctx.Watch(msg.target)
		case *actor.Terminated:
			fmt.Printf("ECHO:Actor %v terminated \n", msg.Who.String())
		default:
			fmt.Printf("ECHO: %+v\n", msg)
		}

	}), "echo")
}

func watch(line string) {
	parts := strings.SplitN(line, " ", 2)

	if len(parts) != 2 {
		fmt.Printf("Wrong number of arguments for `watch`. expected: pid\n")
	} else {

		pidStr := parts[1]
		x := strings.SplitN(pidStr, "/", 2)
		address := x[0]
		id := x[1]
		pid := actor.NewPID(address, id)
		rootContext.Send(echoPID, &watchRequest{
			target: pid,
		})
	}
}

func tell(line string) {
	parts := strings.SplitN(line, " ", 4)

	if len(parts) != 4 {
		fmt.Printf("Wrong number of arguments for `tell`. expected: pid type-name json\n")
	} else {

		pidStr := parts[1]
		typeNameStr := parts[2]
		jsonStr := parts[3]

		x := strings.SplitN(pidStr, "/", 2)
		address := x[0]
		id := x[1]

		err := parseJson(jsonStr)
		if err == nil {
			m := &remote.JsonMessage{
				Json:     jsonStr,
				TypeName: typeNameStr,
			}
			pid := actor.NewPID(address, id)
			remote.SendMessage(pid, nil, m, nil, 1)
		} else {
			fmt.Printf("Invalid JSON payload: %v\n", err)
		}
	}
}

type watchRequest struct {
	target *actor.PID
}

func parseJson(s string) error {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js)
}
