package enrollment

import (
	"fmt"
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/docker/infrakit/pkg/controller/enrollment/types"
	"github.com/docker/infrakit/pkg/spi/instance"
)

// keyFunc is a function that extracts the key from the description
type keyFunc func(instance.Description) (string, error)

// index processes the list to create a map of the key value to the associated instance.
func index(list instance.Descriptions, getKey keyFunc) (map[string]instance.Description, mapset.Set, error) {
	// Track errors and return what could be indexed
	var e error
	index := map[string]instance.Description{}
	this := mapset.NewSet()
	for _, n := range list {
		key, err := getKey(n)
		if err != nil {
			log.Error("cannot index entry", "instance.ID", n.ID, "instance.tags", n.Tags, "err", err)
			e = err
			continue
		}
		this.Add(key)
		index[key] = n
	}
	return index, this, e
}

// Difference returns a list of specs that is not in the receiver.
func Difference(list instance.Descriptions, listKeyFunc keyFunc,
	other instance.Descriptions, otherKeyFunc keyFunc) instance.Descriptions {
	this, thisSet, _ := index(list, listKeyFunc)
	_, thatSet, _ := index(other, otherKeyFunc)
	return toDescriptions(thisSet.Difference(thatSet), this)
}

func toDescriptions(set mapset.Set, index map[string]instance.Description) instance.Descriptions {
	out := instance.Descriptions{}
	for n := range set.Iter() {
		out = append(out, index[n.(string)])
	}
	sort.Sort(out)
	return out
}

// Delta computes the changes necessary to make the list match other:
// 1. the add Descriptions are entries to add to other
// 2. the remove Descriptions are entries to remove from other
func Delta(list instance.Descriptions, listKeyFunc keyFunc, listParseOp string,
	other instance.Descriptions, otherKeyFunc keyFunc, otherParseOp string) (add instance.Descriptions, remove instance.Descriptions) {

	sort.Sort(instance.Descriptions(list))
	sort.Sort(instance.Descriptions(other))

	this, thisSet, errList := index(list, listKeyFunc)
	that, thatSet, errOther := index(other, otherKeyFunc)

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
		removeSet := thatSet.Difference(thisSet)
		remove = toDescriptions(removeSet, that)
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
		addSet := thisSet.Difference(thatSet)
		add = toDescriptions(addSet, this)
	}

	sort.Sort(instance.Descriptions(add))
	sort.Sort(instance.Descriptions(remove))

	return
}
