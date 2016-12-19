package gcp

import "github.com/docker/editions/pkg/loadbalancer"

type gcpResult struct {
	msg string
}

func (r *gcpResult) String() string {
	return r.msg
}

// NewResult creates a new Result
func NewResult(msg string) loadbalancer.Result {
	return &gcpResult{
		msg: msg,
	}
}
