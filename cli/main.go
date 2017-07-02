package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/chzyer/readline"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/remote"
	proto "github.com/gogo/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, completer.Tree("    "))
}

// Function constructor - constructs new function for listing given directory
var completer = readline.NewPrefixCompleter(
	// readline.PcItem("mode",
	// 	readline.PcItem("vi"),
	// 	readline.PcItem("emacs"),
	// ),
	// readline.PcItem("login"),
	// readline.PcItem("say",
	// 	readline.PcItemDynamic(listFiles("./"),
	// 		readline.PcItem("with",
	// 			readline.PcItem("following"),
	// 			readline.PcItem("items"),
	// 		),
	// 	),
	// 	readline.PcItem("hello"),
	// 	readline.PcItem("bye"),
	// ),
	// readline.PcItem("setprompt"),
	// readline.PcItem("setpassword"),
	// readline.PcItem("bye"),
	// readline.PcItem("help"),
	// readline.PcItem("go",
	// 	readline.PcItem("build", readline.PcItem("-o"), readline.PcItem("-v")),
	// 	readline.PcItem("install",
	// 		readline.PcItem("-v"),
	// 		readline.PcItem("-vv"),
	// 		readline.PcItem("-vvv"),
	// 	),
	// 	readline.PcItem("test"),
	// ),
	readline.PcItem("connect"),
	readline.PcItem("tell"),
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

func main() {
	remote.DefaultSerializerID = 1
	remote.Start("127.0.0.1:0")
	actor.SpawnNamed(actor.FromFunc(func(ctx actor.Context) {
		log.Printf("ECHO: %+v", ctx.Message())
	}), "echo")

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

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})

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
		log.Println(line)
		switch {

		case strings.HasPrefix(line, "connect "):
			address := line[8:]
			pid := actor.NewPID(address, "a")
			pid.Tell(&remote.Unit{})
		case strings.HasPrefix(line, "tell "):
			parts := strings.SplitN(line, " ", 4)
			i := parts[1]
			x := strings.SplitN(i, "/", 2)
			address := x[0]
			id := x[1]
			m := &remote.JsonMessage{
				Json:     parts[3],
				TypeName: parts[2],
			}
			pid := actor.NewPID(address, id)
			sid := int32(1)
			remote.SendMessage(pid, m, nil, &sid)

		// case strings.HasPrefix(line, "mode "):
		// 	switch line[5:] {
		// 	case "vi":
		// 		l.SetVimMode(true)
		// 	case "emacs":
		// 		l.SetVimMode(false)
		// 	default:
		// 		println("invalid mode:", line[5:])
		// 	}
		// case line == "mode":
		// 	if l.IsVimMode() {
		// 		println("current mode: vim")
		// 	} else {
		// 		println("current mode: emacs")
		// 	}
		// case line == "login":
		// 	pswd, err := l.ReadPassword("please enter your password: ")
		// 	if err != nil {
		// 		break
		// 	}
		// 	println("you enter:", strconv.Quote(string(pswd)))
		// case line == "help":
		// 	usage(l.Stderr())
		// case line == "setpassword":
		// 	pswd, err := l.ReadPasswordWithConfig(setPasswordCfg)
		// 	if err == nil {
		// 		println("you set:", strconv.Quote(string(pswd)))
		// 	}
		// case strings.HasPrefix(line, "setprompt"):
		// 	if len(line) <= 10 {
		// 		log.Println("setprompt <prompt>")
		// 		break
		// 	}
		// 	l.SetPrompt(line[10:])
		// case strings.HasPrefix(line, "say"):
		// 	line := strings.TrimSpace(line[3:])
		// 	if len(line) == 0 {
		// 		log.Println("say what?")
		// 		break
		// 	}
		// 	go func() {
		// 		for range time.Tick(time.Second) {
		// 			log.Println(line)
		// 		}
		// 	}()
		case line == "exit":
			goto exit
		case line == "":
		default:
			log.Println("you said:", strconv.Quote(line))
		}
	}
exit:
}
