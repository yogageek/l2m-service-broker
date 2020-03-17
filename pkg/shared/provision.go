package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/recorder"
)

var (
	//init http request
	client         = &http.Client{}
	provisionFaied = "Provision Faied"
)

func lwmMapInstance(request *osb.ProvisionRequest) map[string]interface{} {
	return map[string]interface{}{
		"subscriptionId": request.OrganizationGUID,
		//"space_guid":        request.SpaceGUID,
		"instanceId": request.InstanceID,
		"parameters": request.Parameters,
	}
}

// add header automatically for each HTTP request
func reqWithHeader(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", os.Getenv("TOKEN"))
	return req, nil
}

func (s *mongoShared) Provision(request *osb.ProvisionRequest, c *broker.RequestContext) (*broker.ProvisionResponse, error) {
	uri, err := recorder.FetchURL()
	if err != nil {
		return nil, err
	}
	glog.Info(uri)

	//set POST requestBody
	reqMap := lwmMapInstance(request)
	glog.Info("reqMap->", reqMap)
	//convert byte json to string json
	bytesJSON, err := json.MarshalIndent(reqMap, "", "")
	if err != nil {
		fmt.Println(err)
	}
	strJSON := strings.NewReader(string(bytesJSON))
	glog.Info("strJSON->", strJSON)

	//調用添加header方法並放入request參數
	//外部
	// req, _ := reqWithHeader("POST", fmt.Sprintf("http://%s/tenant/tenants/createdByServiceBroker", uri), strJSON)
	//內部
	var createAPI string
	if os.Getenv("CREATE_API") == "" {
		createAPI = "/v1/tenants/createdByServiceBroker"
	}
	reqpoint := fmt.Sprintf("http://%s%s", uri, createAPI)
	fmt.Println("provision request api:", reqpoint)
	req, _ := reqWithHeader("POST", reqpoint, strJSON)

	//送出請求
	res, err := client.Do(req)
	// This section will not handle any error, should pass error without mutate.
	if err != nil {
		fmt.Println("res, err := client.Do(req) err:", err)
		return nil, err
	}

	//get returned body
	defer res.Body.Close()
	resBody, _ := ioutil.ReadAll(res.Body)
	resBodyStr := string(resBody)
	if len(resBodyStr) > 0 || resBodyStr != "" {
		glog.Info("lwm server returned:", resBodyStr)
	} else {
		glog.Info("lwm server didn't return anything.")
	}
	//拆解 returned body
	m := make(map[string]interface{})                   // var respJSON map[string]interface{} //這樣宣告也可以
	if err := json.Unmarshal(resBody, &m); err != nil { // err := json.Unmarshal(bytesRtnJson, &respJSON)	+ 判斷
		spew.Dump(m)
		glog.Error(err)
	}

	//check if provision success
	if res.StatusCode != 200 {
		fmt.Println("provision failed, res.StatusCode != 200")
		errString := fmt.Sprintf("lwm server returned: %v, %v", res.StatusCode, resBodyStr)
		err = errors.New(errString)
		//handle all of err to return above
		if err != nil {
			errString := err.Error()
			return nil, osb.HTTPStatusCodeError{
				ErrorMessage: &provisionFaied,
				StatusCode:   res.StatusCode,
				Description:  &errString,
			}
		}
	}
	// else = 200
	// m["dashboard_url"]-> 获取json中的屬性dashboard_url(注意大小寫都要一樣)
	dashboardURL := m["dashboardUrl"].(string)

	// response := broker.ProvisionResponse{}
	response := broker.ProvisionResponse{osb.ProvisionResponse{DashboardURL: &dashboardURL}, false} //待測試
	fmt.Printf("[Provision Success] dashboardUrl=%s", *response.DashboardURL)
	return &response, nil
}
