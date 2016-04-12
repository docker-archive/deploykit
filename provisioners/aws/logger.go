package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"log"
	"os"
)

type logger struct {
	logger *log.Logger
}

func (l logger) Log(args ...interface{}) {
	l.logger.Println(args...)
}

func getLogger() aws.Logger {
	return &logger{
		logger: log.New(os.Stderr, "", log.LstdFlags),
	}
}
