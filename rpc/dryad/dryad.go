// Generated by go-rpcgen. Do not modify.

package dryad

import (
	"crypto/rsa"
	. "git.tizen.org/tools/boruta"
	"net/rpc"
)

// DryadService is generated service for Dryad interface.
type DryadService struct {
	impl Dryad
}

// NewDryadService creates a new DryadService instance.
func NewDryadService(impl Dryad) *DryadService {
	return &DryadService{impl}
}

// RegisterDryadService registers impl in server.
func RegisterDryadService(server *rpc.Server, impl Dryad) error {
	return server.RegisterName("Dryad", NewDryadService(impl))
}

// DryadPutInMaintenanceRequest is a helper structure for PutInMaintenance method.
type DryadPutInMaintenanceRequest struct {
	Msg string
}

// DryadPutInMaintenanceResponse is a helper structure for PutInMaintenance method.
type DryadPutInMaintenanceResponse struct {
}

// PutInMaintenance is RPC implementation of PutInMaintenance calling it.
func (s *DryadService) PutInMaintenance(request *DryadPutInMaintenanceRequest, response *DryadPutInMaintenanceResponse) (err error) {
	err = s.impl.PutInMaintenance(request.Msg)
	return
}

// DryadPrepareRequest is a helper structure for Prepare method.
type DryadPrepareRequest struct {
}

// DryadPrepareResponse is a helper structure for Prepare method.
type DryadPrepareResponse struct {
	Key *rsa.PrivateKey
}

// Prepare is RPC implementation of Prepare calling it.
func (s *DryadService) Prepare(request *DryadPrepareRequest, response *DryadPrepareResponse) (err error) {
	response.Key, err = s.impl.Prepare()
	return
}

// DryadHealthcheckRequest is a helper structure for Healthcheck method.
type DryadHealthcheckRequest struct {
}

// DryadHealthcheckResponse is a helper structure for Healthcheck method.
type DryadHealthcheckResponse struct {
}

// Healthcheck is RPC implementation of Healthcheck calling it.
func (s *DryadService) Healthcheck(request *DryadHealthcheckRequest, response *DryadHealthcheckResponse) (err error) {
	err = s.impl.Healthcheck()
	return
}

// DryadClient is generated client for Dryad interface.
type DryadClient struct {
	client *rpc.Client
}

// DialDryadClient connects to addr and creates a new DryadClient instance.
func DialDryadClient(addr string) (*DryadClient, error) {
	client, err := rpc.Dial("tcp", addr)
	return &DryadClient{client}, err
}

// NewDryadClient creates a new DryadClient instance.
func NewDryadClient(client *rpc.Client) *DryadClient {
	return &DryadClient{client}
}

// Close terminates the connection.
func (_c *DryadClient) Close() error {
	return _c.client.Close()
}

// PutInMaintenance is part of implementation of Dryad calling corresponding method on RPC server.
func (_c *DryadClient) PutInMaintenance(msg string) (err error) {
	_request := &DryadPutInMaintenanceRequest{msg}
	_response := &DryadPutInMaintenanceResponse{}
	err = _c.client.Call("Dryad.PutInMaintenance", _request, _response)
	return err
}

// Prepare is part of implementation of Dryad calling corresponding method on RPC server.
func (_c *DryadClient) Prepare() (key *rsa.PrivateKey, err error) {
	_request := &DryadPrepareRequest{}
	_response := &DryadPrepareResponse{}
	err = _c.client.Call("Dryad.Prepare", _request, _response)
	return _response.Key, err
}

// Healthcheck is part of implementation of Dryad calling corresponding method on RPC server.
func (_c *DryadClient) Healthcheck() (err error) {
	_request := &DryadHealthcheckRequest{}
	_response := &DryadHealthcheckResponse{}
	err = _c.client.Call("Dryad.Healthcheck", _request, _response)
	return err
}
