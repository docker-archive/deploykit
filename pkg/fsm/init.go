package fsm

import (
	logutil "github.com/docker/infrakit/pkg/log"
)

var (
	log     = logutil.New("module", "core/fsm")
	debugV  = logutil.V(600)
	debugV2 = logutil.V(1200)
)
