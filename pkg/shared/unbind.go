package shared

import (
	"fmt"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	"github.com/prometheus/common/log"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/util"
)

func (s *mongoShared) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {
	var (
		uri, uri_external, zk, kfkshell string
	)
	// uri, uri_external, zk, kfkshell, err := recorder.FetchURL()
	// if err != nil {
	// 	return nil, err
	// }
	fmt.Printf("uri:%v,uri_external:%v,zk:%v,kfkshell:%v,", uri, uri_external, zk, kfkshell)

	//0. Create connection
	admin, err := util.GetClusterAdmin(uri) //確認getClusterAdmin沒用指標位置寫法是否會造成影響
	if err != nil {                         //連線錯誤
		errString := err.Error()
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusServiceUnavailable,
			Description:  &uri,
		}
	}

	var (
		instanceID = request.InstanceID
		bindingID  = request.BindingID
		filter     = sarama.AclFilter{
			ResourceType:              sarama.AclResourceAny,
			PermissionType:            sarama.AclPermissionAllow, //must put the correct info
			Operation:                 sarama.AclOperationAny,    //must put the correct info (any means choose all of acls)
			ResourcePatternTypeFilter: sarama.AclPatternLiteral,  // a must only for clusterAdmin.deleteAcl
			ResourceName:              &bindingID,                //if not defined, all of resouces will be delete!
		}
	)
	// fmt.Printf("filter to delete:%+v\n", filter)

	//use describe topic first to check if topic exist!!!!!!! if not, return err.
	//to avoid deletetopic failed but delete acl success

	//1.delete topic - modify to if topic not exist then return err
	err = admin.DeleteTopic(bindingID)
	if err != nil {
		errString := err.Error()
		log.Errorf("delete topic failed, %v", errString)
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusInternalServerError,
			Description:  &UnbindFailed,
		}
	}

	// fmt.Println("--------------ListAcls-------------------")
	// listacls, err := admin.ListAcls(filter)
	// if err != nil {
	// 	log.Println("error in connect: ", err)
	// 	return err
	// }
	// for i, acls := range listacls {
	// 	fmt.Printf("[%v] acl=%+v\n", i, acls)
	// }

	//2.delete acl
	_, err = admin.DeleteACL(filter, false)
	if err != nil {
		glog.Errorf("delete acl failed: %v", err)
		errString := err.Error()
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusInternalServerError,
			Description:  &UnbindFailed,
		}
	}
	err = admin.Close()
	if err != nil {
		fmt.Println("error in close admin, ", err)
	}

	// check which aclls are deleted
	// fmt.Println("===Deleted ACLs===")
	// for i, acls := range deletedACLs {
	// 	fmt.Printf("[%v]\nacls.Resource=%+v\n", i, acls.Resource)
	// 	fmt.Printf("acls.Acl=%+v\n", acls.Acl)
	// }

	// fmt.Println("--------------check acls after delete-------------------------")
	// listacls, err = admin.ListAcls(filter)
	// if err != nil {
	// 	log.Println("error in connect: ", err)
	// 	return err
	// }
	// for i, acls := range listacls {
	// 	fmt.Printf("[%v] acl=%+v\n", i, acls)
	// }

	fmt.Printf("[Unbind Success] User=%v , Topic=%v\n", instanceID, bindingID)

	// Unbind response doesnt return credential
	response := broker.UnbindResponse{}
	return &response, nil
}
