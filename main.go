package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"
)

type logStruct struct {
	isError bool
	command []string
	text    string
}

type jobStruct struct {
	command []string
	input   string
}

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "command",
			Aliases:  []string{"c"},
			Usage:    "workers command",
			Required: true,
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Usage:   "number of workers",
			Value:   runtime.NumCPU(),
		},
		&cli.StringFlag{
			Name:    "replace",
			Aliases: []string{"r"},
			Usage:   "replace value with STDIN line",
			Value:   "{}",
		},
		&cli.BoolFlag{
			Name:    "log",
			Aliases: []string{"l"},
			Usage:   "put all log in STDERR",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "echo",
			Aliases: []string{"e"},
			Usage:   "echo STDIN",
			Value:   false,
		},
	}

	app.Action = func(c *cli.Context) error {
		logChan := make(chan logStruct)
		jobChan := make(chan jobStruct)
		jobsDone := make(chan bool)
		nJobs := 0
		workers := c.Int("workers")
		command := c.String("command")
		replace := c.String("replace")
		echo := c.Bool("echo")

		go logWorker(logChan, c.Bool("log"))

		if workers <= 0 {
			return errors.New("Must be 1 or more workers")
		}

		for i := 0; i < workers; i++ {
			go worker(jobChan, jobsDone, logChan)
		}

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			nJobs++
			input := scanner.Text()
			var in string
			if strings.Contains(command, replace) {
				in = strings.ReplaceAll(command, replace, input)
			} else {
				in = fmt.Sprint(command, " ", input)
			}

			if echo {
				jobChan <- jobStruct{
					command: strings.Split(in, " "),
					input:   input,
				}
			} else {
				jobChan <- jobStruct{
					command: strings.Split(in, " "),
					input:   "",
				}
			}
		}

		done := 0
		for range jobsDone {
			done++
			if done == nJobs {
				break
			}
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

func worker(jobs <-chan jobStruct, ends chan bool, logChan chan logStruct) {
	for job := range jobs {
		cmd := exec.Command(job.command[0], job.command[1:]...)

		var outb, errb bytes.Buffer

		cmd.Stdout = &outb
		cmd.Stderr = &errb

		err := cmd.Run()

		if o := outb.String(); o != "" && o != "<nil>" {
			logChan <- logStruct{
				command: job.command,
				isError: false,
				text:    string(o),
			}
		}

		if e := errb.String(); (e != "" && e != "<nil>") || err != nil {
			logChan <- logStruct{
				command: job.command,
				isError: true,
				text:    string(e),
			}
		}

		if job.input != "" {
			fmt.Fprintln(os.Stdout, job.input)
		}

		go func() {
			ends <- true
		}()
	}
}

func logWorker(logs <-chan logStruct, log bool) {
	for l := range logs {
		if log {
			if l.isError {
				fmt.Fprintf(os.Stderr, "[Erro] %s -> %s", l.command, l.text)
			} else {
				fmt.Fprintf(os.Stderr, "[Info] %s -> %s", l.command, l.text)
			}
		}
	}
}
