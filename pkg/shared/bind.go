package shared

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Shopify/sarama"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/pkg/util"
)

func (s *mongoShared) Bind(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {
	var (
		uri, uri_external, zk, kfkshell string
	)
	//會自動使用傳進來的instanceid去db尋找相對應的url
	// uri, uri_external, zk, kfkshell, err := recorder.FetchURL() //會自動使用傳進來的instanceid去db尋找相對應的url
	// if err != nil {
	// 	return nil, err
	// }
	fmt.Printf("uri:%v,uri_external:%v,zk:%v,kfkshell:%v,", uri, uri_external, zk, kfkshell)

	var (
		instanceID     = request.InstanceID
		bindingID      = request.BindingID
		retentionBytes = util.RetentionBytes
		segmentBytes   = util.SegmentBytes
	)

	//0. Create connection
	admin, err := util.GetClusterAdmin(uri) //確認getClusterAdmin沒用指標位置寫法是否會造成影響
	if err != nil {                         //連線錯誤
		errString := err.Error()
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusServiceUnavailable,
			Description:  &uri, //才知道是哪個平台uri
		}
	}

	//1. CREATE topic name as bindgingID
	configEntries := map[string]*string{
		"retention.bytes": &retentionBytes,
		"segment.bytes":   &segmentBytes,
	}
	err = admin.CreateTopic(bindingID, &sarama.TopicDetail{
		NumPartitions:     int32(util.Partitions),
		ReplicationFactor: int16(util.Replications),
		ConfigEntries:     configEntries,
	}, false)
	if err != nil {
		errString := err.Error()
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusInternalServerError,
			Description:  &BindingFailed,
		}
	}

	//2. Create userACL for Topic
	//Consumer ACL
	resouceGroup := sarama.Resource{
		ResourceType: sarama.AclResourceGroup,
		ResourceName: bindingID,
	}
	//Producer ACL
	resouceTopic := sarama.Resource{
		ResourceType: sarama.AclResourceTopic,
		ResourceName: bindingID,
	}
	user := "User:" + instanceID //define principal
	setacl := func(op sarama.AclOperation) sarama.Acl {
		acl := sarama.Acl{
			Principal:      user,
			Host:           "*",
			Operation:      op,
			PermissionType: sarama.AclPermissionAllow,
		}
		return acl
	}

	// give 3 allowance to user
	aclRead := setacl(sarama.AclOperationRead)
	aclWrite := setacl(sarama.AclOperationWrite)
	aclDescribe := setacl(sarama.AclOperationDescribe)

	//create topic acl
	var acls = []sarama.Acl{aclRead, aclWrite, aclDescribe}
	for _, acl := range acls {
		err = admin.CreateACL(resouceTopic, acl)
		if err != nil {
			log.Println("CreateACL err: ", err)
			// return err
		}
	}
	//create group acl
	err = admin.CreateACL(resouceGroup, aclRead)
	if err != nil {
		log.Println("CreateACL err: ", err)
		// return err
	}
	if err != nil {
		errString := err.Error()
		return nil, osb.HTTPStatusCodeError{
			ErrorMessage: &errString,
			StatusCode:   http.StatusInternalServerError,
			Description:  &BindingFailed,
		}
	}
	err = admin.Close()
	if err != nil {
		fmt.Println("error in close admin, ", err)
	}

	fmt.Printf("[Bind Success] User=%v , Topic=%v\n", instanceID, bindingID)

	//response
	var (
		kafkaBrokers = uri_external
		topic        = bindingID
		userID       = instanceID
		userPassword = util.GetMD5Password(instanceID)
	)

	response := broker.BindResponse{
		BindResponse: osb.BindResponse{
			Credentials: mapInstance(kafkaBrokers, topic, userID, userPassword),
		},
	}

	return &response, nil
}
