package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/holys/goredis"
	"github.com/peterh/liner"
)

var hostname = flag.String("h", "127.0.0.1", "Server hostname")
var port = flag.Int("p", 6379, "Server server port")
var socket = flag.String("s", "", "Server socket. (overwrites hostname and port)")
var dbn = flag.Int("n", 0, "Database number(default 0)")
var auth = flag.String("a", "", "Password to use when connecting to the server")

var (
	line        *liner.State
	historyPath = path.Join(os.Getenv("HOME"), ".rediscli_history") // $HOME/.rediscli_history
)

func main() {
	flag.Parse()

	line = liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	setCompletionHandler()
	loadHisotry()

	defer saveHisotry()

	var addr string
	if len(*socket) > 0 {
		addr = *socket
	} else {
		addr = fmt.Sprintf("%s:%d", *hostname, *port)
	}

	c := goredis.NewClient(addr, "")
	c.SetMaxIdleConns(1)
	sendSelect(c, *dbn)
	sendAuth(c, *auth)

	reg, _ := regexp.Compile(`'.*?'|".*?"|\S+`)

	prompt := ""

	for {
		if *dbn > 0 && *dbn < 16 {
			prompt = fmt.Sprintf("%s[%d]> ", addr, *dbn)
		} else {
			prompt = fmt.Sprintf("%s> ", addr)
		}

		cmd, err := line.Prompt(prompt)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			return
		}

		cmds := reg.FindAllString(cmd, -1)
		if len(cmds) == 0 {
			continue
		} else {
			line.AppendHistory(cmd)

			args := make([]interface{}, len(cmds[1:]))

			for i := range args {
				args[i] = strings.Trim(string(cmds[1+i]), "\"'")
			}

			cmd := strings.ToLower(cmds[0])
			if cmd == "help" || cmd == "?" {
				printHelp(cmds)
			} else {
				r, err := c.Do(cmds[0], args...)

				if err == nil && strings.ToLower(cmds[0]) == "select" {
					*dbn, _ = strconv.Atoi(cmds[1])
				}

				if err != nil {
					fmt.Printf("%s", err.Error())
				} else {
					if cmd == "info" {
						printInfo(r.([]byte))
					} else {
						printReply(0, r)
					}
				}

				fmt.Printf("\n")
			}

		}
	}
}

func printInfo(s []byte) {
	fmt.Printf("%s", s)
}

func printReply(level int, reply interface{}) {
	switch reply := reply.(type) {
	case int64:
		fmt.Printf("(integer) %d", reply)
	case string:
		fmt.Printf("%s", reply)
	case []byte:
		fmt.Printf("%q", reply)
	case nil:
		fmt.Printf("(nil)")
	case goredis.Error:
		fmt.Printf("%s", string(reply))
	case []interface{}:
		for i, v := range reply {
			if i != 0 {
				fmt.Printf("%s", strings.Repeat(" ", level*4))
			}

			s := fmt.Sprintf("%d) ", i+1)
			fmt.Printf("%-4s", s)

			printReply(level+1, v)
			if i != len(reply)-1 {
				fmt.Printf("\n")
			}
		}
	default:
		fmt.Printf("invalid reply")
	}
}

func printGenericHelp() {
	msg :=
		`redis-cli
Type:	"help <command>" for help on <command>
	`
	fmt.Println(msg)
}

func printCommandHelp(arr []string) {
	fmt.Println()
	fmt.Printf("\t%s %s \n", arr[0], arr[1])
	fmt.Printf("\tGroup: %s \n", arr[2])
	fmt.Println()
}

func printHelp(cmds []string) {
	args := cmds[1:]
	if len(args) == 0 {
		printGenericHelp()
	} else if len(args) > 1 {
		fmt.Println()
	} else {
		cmd := strings.ToUpper(args[0])
		for i := 0; i < len(helpCommands); i++ {
			if helpCommands[i][0] == cmd {
				printCommandHelp(helpCommands[i])
			}
		}
	}
}

func sendSelect(client *goredis.Client, index int) {
	if index == 0 {
		// do nothing
		return
	}
	if index > 16 || index < 0 {
		index = 0
		fmt.Println("index out of range, should less than 16")
	}
	_, err := client.Do("SELECT", index)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
}

func sendAuth(client *goredis.Client, passwd string) {
	if passwd == "" {
		// do nothing
		return
	}

	_, err := client.Do("AUTH", passwd)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
}

func setCompletionHandler() {
	line.SetCompleter(func(line string) (c []string) {
		for _, i := range helpCommands {
			if strings.HasPrefix(i[0], strings.ToLower(line)) {
				c = append(c, i[0])
			}
		}
		return
	})
}

func loadHisotry() {
	if f, err := os.Open(historyPath); err == nil {
		line.ReadHistory(f)
		f.Close()
	}
}

func saveHisotry() {
	if f, err := os.Create(historyPath); err != nil {
		fmt.Printf("Error writing history file: ", err)
	} else {
		line.WriteHistory(f)
		f.Close()
	}
}