package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	manager_discovery "github.com/docker/infrakit/pkg/manager/discovery"
	"github.com/docker/infrakit/pkg/util/exec"
)

// terraformApply starts a goroutine that executes "terraform apply" at the
// configured freqency; if the goroutine is already running then the sleeping
// is interrupted
func (p *plugin) terraformApply() error {
	if p.pretend {
		return nil
	}

	p.applyLock.Lock()
	defer p.applyLock.Unlock()

	if p.applying {
		select {
		case p.pollChannel <- true:
			log.Infoln("Successfully interrupted terraform apply goroutine")
		default:
			log.Infoln("Polling channel is full, not interrupting")
		}
		return nil
	}

	p.pollChannel = make(chan bool, 1)
	go func() {
		initial := true
		for {
			execute, err := p.continuePolling()
			// Only poll is there was no error; if there was an error re-try next cycle
			if err != nil {
				log.Errorf("Failed to determine if polling should continue: %v", err)
			} else {
				if !execute {
					p.applyLock.Lock()
					p.applying = false
					p.applyLock.Unlock()
					log.Infof("Exiting terraform apply gorouting")
					return
				}

				attempted, err := p.doTerraformApply(initial)
				initial = false
				if err != nil {
					log.Errorf("Executing 'terraform apply' failed: %v", err)
				}
				if !attempted {
					log.Infof("Can't acquire apply lock, waiting %v seconds", p.pollInterval)
				}
			}

			select {
			case <-p.pollChannel:
				// Interrupted, use same initial delay so that more than a single delta
				// can be processed
				initial = true
				break
			case <-time.After(p.pollInterval):
				break
			}
		}
	}()

	p.applying = true
	return nil
}

// doTerraformApply executes "terraform apply" if it can aquire the lock. Returns
// true/false if the command was executed and an error
func (p *plugin) doTerraformApply(initial bool) (bool, error) {
	if err := p.lock.TryLock(); err == nil {
		defer p.lock.Unlock()
		// The trigger for the initial apply is typically from a group commit, sleep
		// for a few seconds so that multiple .tf.json files have time to be created
		if initial {
			time.Sleep(time.Second * 5)
		}
		log.Infoln("Applying plan")
		command := exec.Command(`terraform apply`).InheritEnvs(true).WithDir(p.Dir)
		err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start()
		if err != nil {
			return true, err
		}
		return true, command.Wait()
	}
	return false, nil
}

// continuePolling returns true if polling should continue; this happens if either
// the plugin is configured to be standalone or if the associated manager plugin
// is the current leader.
func (p *plugin) continuePolling() (bool, error) {
	// If there is no lookup func then the plugin is running standalone
	if p.pluginLookup == nil {
		return true, nil
	}
	manager, err := manager_discovery.Locate(p.pluginLookup)
	if err != nil {
		return true, err
	}
	isLeader, err := manager.IsLeader()
	if err != nil {
		return true, err
	}
	if isLeader {
		log.Debugf("Manager leadership has not changed, continuing apply polling loop")
	} else {
		log.Infof("No longer running on leader, NOT continuing apply polling loop")
	}
	return isLeader, nil
}
