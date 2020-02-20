package broker

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/recorder"
	_ "advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/shared"
	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/util"
)

//main() will init this
func NewService(o Options) (*BusinessLogic, error) {
	return &BusinessLogic{
		async:     o.Async,
		instances: make(map[string]*serviceInstance, 10),
	}, nil
}

type BusinessLogic struct {
	// Indicates if the broker should handle the requests asynchronously.
	async bool

	instances map[string]*serviceInstance
}

type Message struct {
	InstanceID string `json:"InstanceID"`
	BindingID  string `json:"BindingID"`
	Database   string `json:"Database"`
	Username   string `json:"Username"`
}

var (
	_ broker.Interface = &BusinessLogic{}
)

func isTrue() *bool {
	b := true
	return &b
}

func isFalse() *bool {
	b := false
	return &b
}

func (b *BusinessLogic) GetCatalog(c *broker.RequestContext) (*broker.CatalogResponse, error) {
	// v2
	// DEPRECATED
	// check if header contains wrong header or no header
	// if statusCode, ok := headerVerifier(c); !ok {
	// 	ctype := fmt.Sprintf("%+v", c.Request.Header)
	// 	err := osb.HTTPStatusCodeError{
	// 		StatusCode:   statusCode,
	// 		ErrorMessage: &mismatchHeader,
	// 		Description:  &ctype,
	// 	}
	// 	glog.Error(err)
	// 	return nil, err
	// }

	response := &broker.CatalogResponse{}
	osbResponse := &osb.CatalogResponse{
		Services: []osb.Service{
			{
				Name:          "lwm2m",
				ID:            serviceID,
				Description:   "Lwm2m Service",
				Bindable:      true,
				PlanUpdatable: isTrue(),
				Metadata: map[string]interface{}{
					"displayName":     "Lwm2m",
					"longDescription": "Lwm2m Service",
					"shareable":       true,
				},
				Plans: []osb.Plan{
					{
						Name:        "Shared",
						ID:          planID,
						Description: "This is a shared Lwm2m plan.  All services use shared broker.",
						Metadata: map[string]interface{}{
							"costs": map[string]interface{}{
								"amount": map[string]interface{}{
									"usd": 0,
								},
								"unit": "MONTHLY",
							},
							"bullets": "Shared Lwm2m server",
							"free":    true,
						},
						Free: isFalse(),
					},
				},
				Tags: []string{
					"lwm2m",
					"document",
				},
				Requires:        []string{},
				DashboardClient: nil,
			},
		},
	}

	glog.V(2).Infof("catalog response: %#+v", osbResponse)
	response.CatalogResponse = *osbResponse

	return response, nil
}

func (b *BusinessLogic) Provision(request *osb.ProvisionRequest, c *broker.RequestContext) (*broker.ProvisionResponse, error) {
	// check if HTTP body has whitespace
	ids := strings.Join([]string{request.ServiceID, request.PlanID, request.OrganizationGUID, request.SpaceGUID}, "")
	if strings.ContainsAny(ids, " ") {
		ctype := fmt.Sprintf("%+v", c.Request.Header)
		errString := fmt.Sprint("request body contains illegal characters")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
			Description:  &ctype,
		}
		glog.Error(err)
		return nil, err
	}

	// 1.4.1
	// yoga modify check if any of parameter below is empty or invalid
	if request.ServiceID != serviceID {
		errString := fmt.Sprint("invalid value on following: service_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.PlanID != planID {
		errString := fmt.Sprint("invalid value on following: plan_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.OrganizationGUID == "" {
		errString := fmt.Sprint("invalid value on following: organization_guid")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.SpaceGUID == "" {
		errString := fmt.Sprint("invalid value on following: space_guid")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	sc := util.GetList(request.PlanID)
	if sc == nil {
		errString := fmt.Sprintf("%s is not registered", request.PlanID)
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	bindString := util.Pad("Provision")
	glog.V(2).Infof(bindString)

	// Check whether instance is exist
	rec := recorder.New()
	defer rec.Cli.Close()

	st := rec.OnProvision(sc.Provision)
	resp, err := st(request, c)

	// This section will not handle any error, should pass error without mutate.
	if err != nil {
		return nil, err
	}

	// The log marks if previous process success without error
	glog.V(2).Infof("%+v\n", Message{InstanceID: request.InstanceID, Database: request.InstanceID})

	if request.AcceptsIncomplete {
		resp.Async = b.async
	}

	return resp, nil
}

func (b *BusinessLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {

	//1.4.1
	if request.ServiceID != serviceID {
		errString := fmt.Sprint("invalid value on following: service_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.PlanID != planID {
		errString := fmt.Sprint("invalid value on following: plan_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	// check if HTTP body has whitespace
	ids := strings.Join([]string{request.ServiceID, request.PlanID}, "")
	if strings.ContainsAny(ids, " ") {
		ctype := fmt.Sprintf("%+v", c.Request.Header)
		errString := fmt.Sprint("request body contains illegal characters")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
			Description:  &ctype,
		}
		glog.Error(err)
		return nil, err
	}

	sc := util.GetList(request.PlanID)
	if sc == nil {
		errString := fmt.Sprintf("%s is not registered", request.PlanID)
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	bindString := util.Pad("Deprovision")
	glog.V(2).Infof(bindString)

	// Check whether instance is exist
	rec := recorder.New()
	defer rec.Cli.Close()

	st := rec.OnDeprovision(sc.Deprovision)
	resp, err := st(request, c)

	// This section will not handle any error, should pass error without mutate.
	if err != nil {
		return nil, err
	}

	// require handler since recorder will probably return resp = nil (by design)
	if resp == nil {
		resp = &broker.DeprovisionResponse{}
		resp.Async = b.async
	}

	// The log marks if previous process success without error
	glog.V(2).Infof("%+v\n", Message{InstanceID: request.InstanceID, Database: request.InstanceID})

	return resp, nil
}

func (b *BusinessLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	return nil, nil
}

func (b *BusinessLogic) Bind(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {
	// 1.4.1
	// yoga modify check if any of parameter below is empty or invalid
	if request.ServiceID != serviceID {
		errString := fmt.Sprint("invalid value on following: service_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.PlanID != planID {
		errString := fmt.Sprint("invalid value on following: plan_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	ids := strings.Join([]string{request.ServiceID, request.PlanID}, "")
	if strings.ContainsAny(ids, " ") {
		errString := fmt.Sprint("request body contains illegal characters")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	sc := util.GetList(request.PlanID)
	if sc == nil {
		errString := fmt.Sprintf("%s is not registered", request.PlanID)
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	bindString := util.Pad("Bind")
	glog.V(2).Infof(bindString)

	// Check whether instance is exist
	rec := recorder.New()
	defer rec.Cli.Close()

	st := rec.OnBind(sc.Bind)
	resp, err := st(request, c)

	// This section will not handle any error, should pass error without mutate.
	if err != nil {
		return nil, err
	}

	// The log marks if previous process success without error
	glog.V(2).Infof("%+v\n", Message{InstanceID: request.InstanceID, BindingID: request.BindingID, Database: request.InstanceID, Username: request.BindingID})

	if request.AcceptsIncomplete {
		resp.Async = b.async
	}

	return resp, nil
}

func (b *BusinessLogic) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {

	// 1.4.1
	// yoga modify check if any of parameter below is empty or invalid
	if request.ServiceID != serviceID {
		errString := fmt.Sprint("invalid value on following: service_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}
	if request.PlanID != planID {
		errString := fmt.Sprint("invalid value on following: plan_id")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	// check if HTTP body has whitespace
	ids := strings.Join([]string{request.ServiceID, request.PlanID}, "")
	if strings.ContainsAny(ids, " ") {
		ctype := fmt.Sprintf("%+v", c.Request.Header)
		errString := fmt.Sprint("request body contains illegal characters")
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
			Description:  &ctype,
		}
		glog.Error(err)
		return nil, err
	}

	sc := util.GetList(request.PlanID)
	if sc == nil {
		errString := fmt.Sprintf("%s is not registered", request.PlanID)
		err := osb.HTTPStatusCodeError{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: &errString,
		}
		glog.Error(err)
		return nil, err
	}

	bindString := util.Pad("Unbind")
	glog.V(2).Infof(bindString)

	// Check whether instance is exist
	rec := recorder.New()
	defer rec.Cli.Close()

	st := rec.OnUnbind(sc.Unbind)
	resp, err := st(request, c)

	// This section will not handle any error, should pass error without mutate.
	if err != nil {
		return nil, err
	}

	// The log marks if previous process success without error
	glog.V(2).Infof("%+v\n", Message{InstanceID: request.InstanceID, BindingID: request.BindingID, Database: request.InstanceID, Username: request.BindingID})

	return resp, nil
}

func (b *BusinessLogic) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*broker.UpdateInstanceResponse, error) {

	response := broker.UpdateInstanceResponse{}
	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	return &response, nil
}

func (b *BusinessLogic) ValidateBrokerAPIVersion(version string) error {
	return nil
}

type serviceInstance struct {
	ID        string
	ServiceID string
	PlanID    string
	Params    map[string]interface{}
}

func (i *serviceInstance) Match(other *serviceInstance) bool {
	return reflect.DeepEqual(i, other)
}

//not sure what for, the real implementation is in bind.go
func mapInstance(databaseName, userName, password string) map[string]interface{} {
	return map[string]interface{}{
		"database": databaseName,
		"username": userName,
		"password": password,
	}
}

// func headerVerifier(c *broker.RequestContext) (int, bool) {
// 	if c.Request.Header == nil {
// 		return http.StatusUnauthorized, false
// 	}

// 	if c.Request.Header.Get("Content-Type") != "application/json" {
// 		return http.StatusUnsupportedMediaType, false
// 	}
// 	return 0, true
// }
