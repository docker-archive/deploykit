package templates

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/spiegela/gorackhd/models"
)

// TemplatesLibGetReader is a Reader for the TemplatesLibGet structure.
type TemplatesLibGetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *TemplatesLibGetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewTemplatesLibGetOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 404:
		result := NewTemplatesLibGetNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		result := NewTemplatesLibGetDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewTemplatesLibGetOK creates a TemplatesLibGetOK with default headers values
func NewTemplatesLibGetOK() *TemplatesLibGetOK {
	return &TemplatesLibGetOK{}
}

/*TemplatesLibGetOK handles this case with default header values.

Successfully retrieved the contents of the specified template
*/
type TemplatesLibGetOK struct {
	Payload TemplatesLibGetOKBody
}

func (o *TemplatesLibGetOK) Error() string {
	return fmt.Sprintf("[GET /templates/library/{name}][%d] templatesLibGetOK  %+v", 200, o.Payload)
}

func (o *TemplatesLibGetOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewTemplatesLibGetNotFound creates a TemplatesLibGetNotFound with default headers values
func NewTemplatesLibGetNotFound() *TemplatesLibGetNotFound {
	return &TemplatesLibGetNotFound{}
}

/*TemplatesLibGetNotFound handles this case with default header values.

The template with specified identifier was not found
*/
type TemplatesLibGetNotFound struct {
	Payload *models.Error
}

func (o *TemplatesLibGetNotFound) Error() string {
	return fmt.Sprintf("[GET /templates/library/{name}][%d] templatesLibGetNotFound  %+v", 404, o.Payload)
}

func (o *TemplatesLibGetNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewTemplatesLibGetDefault creates a TemplatesLibGetDefault with default headers values
func NewTemplatesLibGetDefault(code int) *TemplatesLibGetDefault {
	return &TemplatesLibGetDefault{
		_statusCode: code,
	}
}

/*TemplatesLibGetDefault handles this case with default header values.

Unexpected error
*/
type TemplatesLibGetDefault struct {
	_statusCode int

	Payload *models.Error
}

// Code gets the status code for the templates lib get default response
func (o *TemplatesLibGetDefault) Code() int {
	return o._statusCode
}

func (o *TemplatesLibGetDefault) Error() string {
	return fmt.Sprintf("[GET /templates/library/{name}][%d] templatesLibGet default  %+v", o._statusCode, o.Payload)
}

func (o *TemplatesLibGetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

/*TemplatesLibGetOKBody templates lib get o k body
swagger:model TemplatesLibGetOKBody
*/
type TemplatesLibGetOKBody interface{}
