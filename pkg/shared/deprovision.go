package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/recorder"
)

//使用者停止訂閱後預設保留 30 天，也可以由環境變數更動
//想要恢復訂閱的 instance 需要發 ticket 。
//大SB會將恢復訂閱用戶帶上相同 instance id ，所以小SB provision 的時候，需要有額外邏輯
func (s *mongoShared) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	uri, err := recorder.FetchURL()
	if err != nil {
		return nil, err
	}
	glog.Info(uri)

	//if open deprov setting
	if Deprov {
		//New請求
		client := &http.Client{}
		// //set POST requestBody
		// reqMap := lwmMapInstance(request)
		// glog.Info("reqMap->", reqMap)
		// //convert byte json to string json
		// bytesJSON, err := json.MarshalIndent(reqMap, "", "")
		// if err != nil {
		// 	fmt.Println(err)
		// }
		// strJSON := strings.NewReader(string(bytesJSON))
		// glog.Info("strJSON->", strJSON)

		//改版
		//set delete id
		instanceID := request.InstanceID
		deleteAPI := "/v1/tenants/deletedByServiceBroker/"
		reqpoint := fmt.Sprintf("http://%s%s%s", uri, deleteAPI, instanceID)

		//調用添加header方法並放入request參數
		req, _ := reqWithHeader("DELETE", reqpoint, nil)
		fmt.Println("deprovision request api:", reqpoint)
		//送出請求
		res, err := client.Do(req)
		// This section will not handle any error, should pass error without mutate.
		if err != nil {
			return nil, err
		}

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

		//check if deprovision success
		if res.StatusCode != 200 {
			fmt.Println("deprovision failed, res.StatusCode != 200")
			errString := fmt.Sprintf("lwm server returned: %v, %v", res.StatusCode, resBodyStr)
			// errStr := m["error"].(string)
			err = errors.New(errString)
			if err != nil {
				errString := err.Error()
				return nil, osb.HTTPStatusCodeError{
					ErrorMessage: &DeprovisionFaied,
					StatusCode:   res.StatusCode,
					Description:  &errString,
				}
			}
		}
		// else = 200
		// no need to handle 成功的回覆
		// message := m["message"].(string)
	}
	//如果Deprov沒開 一律return true
	response := broker.DeprovisionResponse{}
	return &response, nil
}
