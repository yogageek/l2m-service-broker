package shared

import (
	"reflect"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/util"
)

type mongoShared struct {
	// Indicates if the broker should handle the requests asynchronously.
	async bool
}

//here to implement broker/logic.go GetCatalog()
func (s *mongoShared) GetCatalog(c *broker.RequestContext) (*broker.CatalogResponse, error) {
	response := &broker.CatalogResponse{}
	osbResponse := &osb.CatalogResponse{}

	glog.V(2).Infof("catalog response: %#+v", osbResponse)
	response.CatalogResponse = *osbResponse

	return response, nil
}

func (s *mongoShared) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	// Your last-operation business logic goes here

	return nil, nil
}

func (s *mongoShared) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*broker.UpdateInstanceResponse, error) {
	// Your logic for updating a service goes here.
	response := broker.UpdateInstanceResponse{}
	if request.AcceptsIncomplete {
		response.Async = s.async
	}

	return &response, nil
}

func (s *mongoShared) ValidateBrokerAPIVersion(version string) error {
	return nil
}

type sharedInstance struct {
	ID        string
	ServiceID string
	PlanID    string
	Params    map[string]interface{}
}

func (i *sharedInstance) Match(other *sharedInstance) bool {
	return reflect.DeepEqual(i, other)
}

//*mongoShared implement broker.Interface(which is broker/logic.go)
func init() { util.Register("lwm2m-iii-shared", &mongoShared{}) }
