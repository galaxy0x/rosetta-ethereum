// Copyright 2020 Coinbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ethereum

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sync/errgroup"
)

const (
	gethLogger       = "gwtc"
	gethStdErrLogger = "gwtc err"
)

// logPipe prints out logs from geth. We don't end when context
// is canceled beacause there are often logs printed after this.
func logPipe(pipe io.ReadCloser, identifier string) error {
	reader := bufio.NewReader(pipe)
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			log.Println("closing", identifier, err)
			return err
		}

		message := strings.ReplaceAll(str, "\n", "")
		log.Println(identifier, message)
	}
}

func Cmd(commandName string, params []string) (string, error) {
	cmd := exec.Command(commandName, params...)
	fmt.Println("Cmd", cmd.Args)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	fmt.Println("Cmd stdout", out.String())
	return out.String(), err
}

// StartGeth starts a geth daemon in another goroutine
// and logs the results to the console.
func StartGeth(ctx context.Context, arguments string, g *errgroup.Group) error {
	log.Println("StartGeth arguments", arguments)
	out, err := Cmd("/app/gwtc", strings.Split(GethInit, " "))
	if err != nil {
		fmt.Println("gwtc init err", err)
		return fmt.Errorf("gwtc init err", err)
	}
	fmt.Println("gwtc init finish", out)
	parsedArgs := strings.Split(arguments, " ")
	cmd := exec.Command(
		"/app/gwtc",
		parsedArgs...,
	) // #nosec G204

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	g.Go(func() error {
		return logPipe(stdout, gethLogger)
	})

	g.Go(func() error {
		return logPipe(stderr, gethStdErrLogger)
	})

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: unable to start geth", err)
	}

	g.Go(func() error {
		<-ctx.Done()

		log.Println("sending interrupt to geth")
		return cmd.Process.Signal(os.Interrupt)
	})

	return cmd.Wait()
}
