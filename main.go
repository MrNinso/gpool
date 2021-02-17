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
	"sync"

	"github.com/urfave/cli/v2"
)

const (
	NULL   = 0
	LOG    = 1
	STDOUT = 2
	STDERR = 3
	PASS   = 4
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
			Usage:   "log in STDERR",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "echo",
			Aliases: []string{"e"},
			Usage:   "echo STDIN",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:  "to-stdout",
			Usage: "echo workers STDOUT and STDERR to STDOUT",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "to-stderr",
			Usage: "echo workers STDOUT and STDERR to STDERR",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "pass-std",
			Usage: "echo workers STDOUT and STDERR",
			Value: false,
		},
	}

	app.Action = func(c *cli.Context) error {
		logChan := make(chan logStruct)
		jobChan := make(chan jobStruct)
		workers := c.Int("workers")
		command := c.Args().Slice()
		replace := c.String("replace")
		echo := c.Bool("echo")
		var w sync.WaitGroup

		logMode := NULL

		if c.Bool("log") {
			logMode = LOG
		} else if c.Bool("to-stdout") {
			logMode = STDOUT
		} else if c.Bool("to-stderr") {
			logMode = STDERR
		} else if c.Bool("pass-std") {
			logMode = PASS
		}

		go logWorker(logChan, logMode)

		if workers <= 0 {
			return errors.New("Must be 1 or more workers")
		}

		for i := 0; i < workers; i++ {
			go worker(jobChan, logChan, &w)
		}

		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			w.Add(1)
			input := scanner.Text()
			var in []string
			for _, a := range command {
				in = append(in, strings.ReplaceAll(a, replace, input))
			}

			if echo {
				jobChan <- jobStruct{
					command: in,
					input:   input,
				}
			} else {
				jobChan <- jobStruct{
					command: in,
					input:   "",
				}
			}
		}

		w.Wait()

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

func worker(jobs <-chan jobStruct, logChan chan logStruct, w *sync.WaitGroup) {
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
				text:    o,
			}
		}

		if e := errb.String(); (e != "" && e != "<nil>") || err != nil {
			logChan <- logStruct{
				command: job.command,
				isError: true,
				text:    e,
			}
		}

		if job.input != "" {
			go func() {
				_, _ = fmt.Fprintln(os.Stdout, job.input)
			}()
		}

		w.Done()
	}
}

func logWorker(logs <-chan logStruct, logMode int) {
	for l := range logs {
		//fmt.Println(l, logMode)
		switch logMode {
		case LOG:
			if l.isError {
				_, _ = fmt.Fprintf(os.Stderr, "[Erro] %s -> %s", l.command, l.text)
			} else {
				_, _ = fmt.Fprintf(os.Stderr, "[Info] %s -> %s", l.command, l.text)
			}
			break
		case STDOUT:
			_, _ = fmt.Fprint(os.Stdout, l.text)
			break
		case STDERR:
			_, _ = fmt.Fprint(os.Stderr, l.text)
			break
		case PASS:
			if l.isError {
				_, _ = fmt.Fprint(os.Stderr, l.text)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, l.text)
			}
			break
		}
	}
}
