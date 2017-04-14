package main

import (
	"math/rand"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/util/exec"
)

func (p *plugin) terraformApply() error {
	if p.pretend {
		return nil
	}

	p.applyLock.Lock()
	defer p.applyLock.Unlock()

	if p.applying {
		return nil
	}

	go func() {
		for {
			if err := p.lock.TryLock(); err == nil {
				defer p.lock.Unlock()
				doTerraformApply(p.Dir)
			}
			log.Debugln("Can't acquire lock, waiting")
			time.Sleep(time.Duration(int64(rand.NormFloat64())%1000) * time.Millisecond)
		}
	}()
	p.applying = true
	return nil
}

func doTerraformApply(dir string) error {
	log.Infoln(time.Now().Format(time.RFC850) + " Applying plan")
	command := exec.Command(`terraform apply`).InheritEnvs(true).WithDir(dir)
	err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start()
	if err != nil {
		return err
	}
	return command.Wait()
}
