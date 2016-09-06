package scaler

import (
	"github.com/docker/libmachete/plugin/group/util"
	"time"
)

type rollingupdate struct {
	oldGroup Scaler
	newGroup Scaler
	count    uint32
}

func (r *rollingupdate) Run() {
	workpool, err := util.NewWorkPool(r.count, r, uint(1))
	if err != nil {
		panic(err)
	}

	workpool.Run()

	// TODO(wfarner): Handle adjusting the group size in conjunction with the update.
}

func (r *rollingupdate) Proceed() {
	// TODO(wfarner): Gate based on health rather than arbitrary time.
	time.Sleep(10 * time.Second)

	// TODO(wfarner): Each step in the update must be persisted.
	r.newGroup.SetSize(r.newGroup.GetSize() + 1)
	r.oldGroup.SetSize(r.oldGroup.GetSize() - 1)
}
