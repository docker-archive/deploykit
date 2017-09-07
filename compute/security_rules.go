package compute

import (
	"fmt"
	"strings"
)

// SecurityRulesClient is a client for the Security Rules functions of the Compute API.
type SecurityRulesClient struct {
	ResourceClient
}

// SecurityRules obtains a SecurityRulesClient which can be used to access to the
// Security Rules functions of the Compute API
func (c *AuthenticatedClient) SecurityRules() *SecurityRulesClient {
	return &SecurityRulesClient{
		ResourceClient: ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "security ip list",
			ContainerPath:       "/secrule/",
			ResourceRootPath:    "/secrule",
		}}
}

// SecurityRuleSpec defines a security rule to be created.
type SecurityRuleSpec struct {
	Name            string `json:"name"`
	SourceList      string `json:"src_list"`
	DestinationList string `json:"dst_list"`
	Application     string `json:"application"`
	Action          string `json:"action"`
	Disabled        bool   `json:"disabled"`
}

// SecurityRuleInfo describes an existing security rule.
type SecurityRuleInfo struct {
	Name            string `json:"name"`
	SourceList      string `json:"src_list"`
	DestinationList string `json:"dst_list"`
	Application     string `json:"application"`
	Action          string `json:"action"`
	Disabled        bool   `json:"disabled"`
	URI             string `json:"uri"`
}

func (c *SecurityRulesClient) getQualifiedListName(name string) string {
	nameParts := strings.Split(name, ":")
	listType := nameParts[0]
	listName := nameParts[1]
	return fmt.Sprintf("%s:%s", listType, c.getQualifiedName(listName))
}

func (c *SecurityRulesClient) unqualifyListName(qualifiedName string) string {
	nameParts := strings.Split(qualifiedName, ":")
	listType := nameParts[0]
	listName := nameParts[1]
	return fmt.Sprintf("%s:%s", listType, c.getUnqualifiedName(listName))
}

func (c *SecurityRulesClient) success(ruleInfo *SecurityRuleInfo) (*SecurityRuleInfo, error) {
	ruleInfo.Name = c.getUnqualifiedName(ruleInfo.Name)
	ruleInfo.SourceList = c.unqualifyListName(ruleInfo.SourceList)
	ruleInfo.DestinationList = c.unqualifyListName(ruleInfo.DestinationList)
	ruleInfo.Application = c.getUnqualifiedName(ruleInfo.Application)
	return ruleInfo, nil
}

// CreateSecurityRule creates a new security rule.
func (c *SecurityRulesClient) CreateSecurityRule(
	name, sourceList, destinationList, application, action string,
	disabled bool) (*SecurityRuleInfo, error) {
	spec := SecurityRuleSpec{
		Name:            c.getQualifiedName(name),
		SourceList:      c.getQualifiedListName(sourceList),
		DestinationList: c.getQualifiedListName(destinationList),
		Application:     c.getQualifiedName(application),
		Action:          action,
		Disabled:        disabled,
	}

	var ruleInfo SecurityRuleInfo
	if err := c.createResource(&spec, &ruleInfo); err != nil {
		return nil, err
	}

	return c.success(&ruleInfo)
}

// GetSecurityRule retrieves the security rule with the given name.
func (c *SecurityRulesClient) GetSecurityRule(name string) (*SecurityRuleInfo, error) {
	var ruleInfo SecurityRuleInfo
	if err := c.getResource(name, &ruleInfo); err != nil {
		return nil, err
	}

	return c.success(&ruleInfo)
}

// UpdateSecurityRule modifies the properties of the security rule with the given name.
func (c *SecurityRulesClient) UpdateSecurityRule(
	name, sourceList, destinationList, application, action string,
	disabled bool) (*SecurityRuleInfo, error) {
	spec := SecurityRuleSpec{
		Name:            c.getQualifiedName(name),
		SourceList:      sourceList,
		DestinationList: destinationList,
		Application:     application,
		Action:          action,
		Disabled:        disabled,
	}

	var ruleInfo SecurityRuleInfo
	if err := c.updateResource(name, &spec, &ruleInfo); err != nil {
		return nil, err
	}

	return c.success(&ruleInfo)
}

// DeleteSecurityRule deletes the security rule with the given name.
func (c *SecurityRulesClient) DeleteSecurityRule(name string) error {
	return c.deleteResource(name)
}
