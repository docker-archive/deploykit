package compute

// IPReservationsClient is a client for the IP Reservations functions of the Compute API.
type IPReservationsClient struct {
	*ResourceClient
}

// IPReservations obtains an IPReservationsClient which can be used to access to the
// IP Reservations functions of the Compute API
func (c *AuthenticatedClient) IPReservations() *IPReservationsClient {
	return &IPReservationsClient{
		ResourceClient: &ResourceClient{
			AuthenticatedClient: c,
			ResourceDescription: "ip reservation",
			ContainerPath:       "/ip/reservation/",
			ResourceRootPath:    "/ip/reservation",
		}}
}

// IPReservationSpec defines an IP reservation to be created.
type IPReservationSpec struct {
	Name       interface{} `json:"name"`
	ParentPool string      `json:"parentpool"`
	Permanent  bool        `json:"permanent"`
	Tags       []string    `json:"tags"`
}

// IPReservationInfo describes an existing IP reservation.
type IPReservationInfo struct {
	Name       string   `json:"name"`
	ParentPool string   `json:"parentpool"`
	Permanent  bool     `json:"permanent"`
	Tags       []string `json:"tags"`
	IP         string   `json:"ip"`
}

func (c *IPReservationsClient) success(result *IPReservationInfo) (*IPReservationInfo, error) {
	c.unqualify(&result.Name)
	return result, nil
}

// CreateIPReservation creates a new IP reservation with the given parentpool, tags and permanent flag.
func (c *IPReservationsClient) CreateIPReservation(parentpool string, permanent bool, tags []string) (*IPReservationInfo, error) {
	spec := IPReservationSpec{
		Name:       nil,
		ParentPool: parentpool,
		Permanent:  permanent,
		Tags:       tags,
	}
	var ipInfo IPReservationInfo
	if err := c.createResource(&spec, &ipInfo); err != nil {
		return nil, err
	}

	return c.success(&ipInfo)
}

// GetIPReservation retrieves the IP reservation with the given name.
func (c *IPReservationsClient) GetIPReservation(name string) (*IPReservationInfo, error) {
	var ipInfo IPReservationInfo
	if err := c.getResource(name, &ipInfo); err != nil {
		return nil, err
	}

	return c.success(&ipInfo)
}

// DeleteIPReservation deletes the IP reservation with the given name.
func (c *IPReservationsClient) DeleteIPReservation(name string) error {
	return c.deleteResource(name)
}
