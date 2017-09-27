package nodes

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// NewNodesGetWorkflowByIDParams creates a new NodesGetWorkflowByIDParams object
// with the default values initialized.
func NewNodesGetWorkflowByIDParams() *NodesGetWorkflowByIDParams {
	var ()
	return &NodesGetWorkflowByIDParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewNodesGetWorkflowByIDParamsWithTimeout creates a new NodesGetWorkflowByIDParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewNodesGetWorkflowByIDParamsWithTimeout(timeout time.Duration) *NodesGetWorkflowByIDParams {
	var ()
	return &NodesGetWorkflowByIDParams{

		timeout: timeout,
	}
}

// NewNodesGetWorkflowByIDParamsWithContext creates a new NodesGetWorkflowByIDParams object
// with the default values initialized, and the ability to set a context for a request
func NewNodesGetWorkflowByIDParamsWithContext(ctx context.Context) *NodesGetWorkflowByIDParams {
	var ()
	return &NodesGetWorkflowByIDParams{

		Context: ctx,
	}
}

// NewNodesGetWorkflowByIDParamsWithHTTPClient creates a new NodesGetWorkflowByIDParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewNodesGetWorkflowByIDParamsWithHTTPClient(client *http.Client) *NodesGetWorkflowByIDParams {
	var ()
	return &NodesGetWorkflowByIDParams{
		HTTPClient: client,
	}
}

/*NodesGetWorkflowByIDParams contains all the parameters to send to the API endpoint
for the nodes get workflow by Id operation typically these are written to a http.Request
*/
type NodesGetWorkflowByIDParams struct {

	/*Active
	  A query string to specify workflow properties to search for

	*/
	Active *bool
	/*Identifier
	  The node identifier

	*/
	Identifier string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) WithTimeout(timeout time.Duration) *NodesGetWorkflowByIDParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) WithContext(ctx context.Context) *NodesGetWorkflowByIDParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) WithHTTPClient(client *http.Client) *NodesGetWorkflowByIDParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithActive adds the active to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) WithActive(active *bool) *NodesGetWorkflowByIDParams {
	o.SetActive(active)
	return o
}

// SetActive adds the active to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) SetActive(active *bool) {
	o.Active = active
}

// WithIdentifier adds the identifier to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) WithIdentifier(identifier string) *NodesGetWorkflowByIDParams {
	o.SetIdentifier(identifier)
	return o
}

// SetIdentifier adds the identifier to the nodes get workflow by Id params
func (o *NodesGetWorkflowByIDParams) SetIdentifier(identifier string) {
	o.Identifier = identifier
}

// WriteToRequest writes these params to a swagger request
func (o *NodesGetWorkflowByIDParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Active != nil {

		// query param active
		var qrActive bool
		if o.Active != nil {
			qrActive = *o.Active
		}
		qActive := swag.FormatBool(qrActive)
		if qActive != "" {
			if err := r.SetQueryParam("active", qActive); err != nil {
				return err
			}
		}

	}

	// path param identifier
	if err := r.SetPathParam("identifier", o.Identifier); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
