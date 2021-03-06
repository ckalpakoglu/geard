package systemd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

var ErrLogWriteTimeout = errors.New("journal: Maximum duration exceeded, timeout")
var ErrLogComplete = errors.New("journal: Closed by caller")

func ProcessLogsForUnit(unit string) (io.ReadCloser, error) {
	cmd := exec.Command("/usr/bin/journalctl", "--since=now", "-q", "-f", "--unit", unit)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return stdout, nil
}

func WriteLogsTo(w io.Writer, unit string, previous int, until <-chan time.Time) error {
	var arg string
	if previous == 0 {
		arg = "--since=now"
	} else {
		arg = fmt.Sprintf("--since=-%d", previous)
	}
	cmd := exec.Command("/usr/bin/journalctl", arg, "-f", "-q", "--unit", unit)
	stdout, errp := cmd.StdoutPipe()
	if errp != nil {
		return errp
	}
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return err
	}

	outch := make(chan error, 1)
	go func() {
		_, err := io.Copy(w, stdout)
		outch <- err
		close(outch)
	}()
	prcch := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		prcch <- err
		close(prcch)
	}()

	var err error
	select {
	case err = <-prcch:
		if err != nil {
			log.Print("journal: Process exited unexpectedly: ", err)
		}
	case err = <-outch:
		if err != nil {
			log.Print("journal: Output closed before process exited: ", err)
		} else {
			log.Print("journal: Write completed")
		}
	case <-until:
		log.Print("journal: Done")
		err = nil
	}

	stdout.Close()
	cmd.Process.Kill()

	select {
	case <-prcch:
	}
	select {
	case <-outch:
	}

	return err
}
