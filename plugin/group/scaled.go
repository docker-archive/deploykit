package scaler

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/libmachete/spi/instance"
	"sync"
)

type scaledGroup struct {
	instancePlugin   instance.Plugin
	memberTags       map[string]string
	provisionRequest json.RawMessage
	provisionTags    map[string]string
	lock             sync.Mutex
}

func (s *scaledGroup) setAdditionalTags(tags map[string]string) {
	s.lock.Lock()
	s.lock.Unlock()

	allTags := map[string]string{}
	for k, v := range tags {
		allTags[k] = v
	}
	for k, v := range s.memberTags {
		allTags[k] = v
	}
	s.provisionTags = allTags
}

func (s *scaledGroup) setProvisionTemplate(provisionRequest json.RawMessage, additionalTags map[string]string) {
	s.lock.Lock()
	s.lock.Unlock()

	s.provisionRequest = provisionRequest
	s.setAdditionalTags(additionalTags)
}

func (s *scaledGroup) CreateOne() {
	id, err := s.instancePlugin.Provision(s.provisionRequest, nil, s.provisionTags)
	if err != nil {
		log.Errorf("Failed to grow group: %s", err)
	} else {
		log.Infof("Created instance %s with tags %v", *id, s.provisionTags)
	}
}

func (s *scaledGroup) Destroy(id instance.ID) error {
	log.Infof("Destroying instance %s", id)
	err := s.instancePlugin.Destroy(id)
	if err != nil {
		log.Errorf("Failed to destroy %s: %s", id, err)
	}
	return err
}

func (s *scaledGroup) describe() ([]instance.Description, error) {
	return s.instancePlugin.DescribeInstances(s.memberTags)
}

func (s *scaledGroup) List() ([]instance.ID, error) {
	descriptions, err := s.describe()
	if err != nil {
		return nil, err
	}

	ids := []instance.ID{}
	for _, desc := range descriptions {
		ids = append(ids, desc.ID)
	}

	return ids, nil
}
