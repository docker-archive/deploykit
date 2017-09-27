package nodes

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/spiegela/gorackhd/models"
)

// NodesGetAllReader is a Reader for the NodesGetAll structure.
type NodesGetAllReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *NodesGetAllReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewNodesGetAllOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 400:
		result := NewNodesGetAllBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		result := NewNodesGetAllDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewNodesGetAllOK creates a NodesGetAllOK with default headers values
func NewNodesGetAllOK() *NodesGetAllOK {
	return &NodesGetAllOK{}
}

/*NodesGetAllOK handles this case with default header values.

Successfully retrieved the list of nodes
*/
type NodesGetAllOK struct {
	Payload []*models.Node20Node
}

func (o *NodesGetAllOK) Error() string {
	return fmt.Sprintf("[GET /nodes][%d] nodesGetAllOK  %+v", 200, o.Payload)
}

func (o *NodesGetAllOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewNodesGetAllBadRequest creates a NodesGetAllBadRequest with default headers values
func NewNodesGetAllBadRequest() *NodesGetAllBadRequest {
	return &NodesGetAllBadRequest{}
}

/*NodesGetAllBadRequest handles this case with default header values.

Bad Request
*/
type NodesGetAllBadRequest struct {
	Payload *models.Error
}

func (o *NodesGetAllBadRequest) Error() string {
	return fmt.Sprintf("[GET /nodes][%d] nodesGetAllBadRequest  %+v", 400, o.Payload)
}

func (o *NodesGetAllBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewNodesGetAllDefault creates a NodesGetAllDefault with default headers values
func NewNodesGetAllDefault(code int) *NodesGetAllDefault {
	return &NodesGetAllDefault{
		_statusCode: code,
	}
}

/*NodesGetAllDefault handles this case with default header values.

Unexpected error
*/
type NodesGetAllDefault struct {
	_statusCode int

	Payload *models.Error
}

// Code gets the status code for the nodes get all default response
func (o *NodesGetAllDefault) Code() int {
	return o._statusCode
}

func (o *NodesGetAllDefault) Error() string {
	return fmt.Sprintf("[GET /nodes][%d] nodesGetAll default  %+v", o._statusCode, o.Payload)
}

func (o *NodesGetAllDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
