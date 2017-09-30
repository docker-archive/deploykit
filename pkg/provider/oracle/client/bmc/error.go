package bmc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
)

// Error is the BMC error defined as: https://docs.us-phoenix-1.oraclecloud.com/api/#/en/iaas/20160918/responses/Error
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error returns the error string
func (e Error) Error() string {
	return fmt.Sprintf("code:%v message:%v", e.Code, e.Message)
}

// NewError returns a pointer to the BMC error
func NewError(resp http.Response) *Error {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	logrus.Debugf("Error body: %s", body)
	bmcError := Error{}
	if err = json.Unmarshal(body, &bmcError); err != nil {
		logrus.Fatalf("Cannot unmarshal Error resp impossible: %s", err)
	}
	return &bmcError
}
