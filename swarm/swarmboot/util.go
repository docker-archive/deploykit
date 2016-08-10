package main

import (
	log "github.com/Sirupsen/logrus"
	"time"
)

func doUntilSuccess(work func() bool, retries int, retryWait time.Duration) bool {
	for i := 0; i < retries; i++ {
		success := work()
		if success {
			return true
		}
		log.Infof("Trying again in %s", retryWait)
		time.Sleep(retryWait)
	}

	return false
}
