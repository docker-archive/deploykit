package enrollment

import (
	"fmt"

	"github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// Delta computes the changes necessary to make the list match other:
// 1. the add Descriptions are entries to add to other
// 2. the remove Descriptions are entries to remove from other
func Delta(list instance.Descriptions, listKeyFunc instance.KeyFunc, listParseOp string,
	other instance.Descriptions, otherKeyFunc instance.KeyFunc, otherParseOp string) (add instance.Descriptions,
	remove instance.Descriptions) {

	thisIndex, errList := list.Index(listKeyFunc)
	thatIndex, errOther := other.Index(otherKeyFunc)

	// If list failed to parse then we either:
	// 1. Remove anything from "other" that either really does not exist in "list" or we cannot
	//    tell if it exists in "list" because we failed to parse the source key
	// 2. No-op
	remove = []instance.Description{}
	processRemove := false
	if errList == nil {
		processRemove = true
	} else {
		switch listParseOp {
		case types.SourceParseErrorDisableDestroy:
			log.Info("EnrollmentDelta",
				"msg",
				fmt.Sprintf("Not removing any enrolled entries due to source parsing error: %v", errList))
		default:
			log.Info("EnrollmentDelta",
				"msg",
				fmt.Sprintf("Destroy is enabled for source parsing error: %v", errList))
			processRemove = true
		}
	}
	if processRemove {
		remove = thatIndex.Select(thatIndex.Keys.Difference(thisIndex.Keys))
	}

	// If other failed to parse then we either:
	// 1. Provision anything in "list" that either really does not exist in "other" or we cannot tell if
	//    it exists in "other" because we failed parse the enrolled source key
	// 2. No-op
	add = []instance.Description{}
	processAdd := false
	if errOther == nil {
		processAdd = true
	} else {
		switch otherParseOp {
		case types.EnrolledParseErrorDisableProvision:
			log.Info("EnrollmentDelta",
				"msg",
				fmt.Sprintf("Not adding any source entries due to enrolled parsing error: %v", errOther))
		default:
			log.Info("EnrollmentDelta",
				"msg",
				fmt.Sprintf("Provision is enabled for enrolled parsing error: %v", errList))
			processAdd = true
		}
	}
	if processAdd {
		add = thisIndex.Select(thisIndex.Keys.Difference(thatIndex.Keys))
	}

	return
}
