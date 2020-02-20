package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"

	"advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/xdg"
	"github.com/Shopify/sarama"
	"github.com/golang/glog"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	"github.com/xdg/scram"
)

var (
	KafkaBrokers   = os.Getenv("KAFKA_BROKERS")
	ZookeeperPeers = os.Getenv("ZOOKEEPER_PEERS")
	RetentionBytes = os.Getenv("RETENTION_BYTES")
	SegmentBytes   = os.Getenv("SEGMENT_BYTES")
	Partitions     = 3
	Replications   = 1
)

var (
	LogicList = map[string]broker.Interface{}
	mutex     sync.Mutex
)

func CallDatabase(name string) string {
	return ""
}

func Pad(msg string) string {
	return fmt.Sprintf(">>%[1]*s<<", -70, fmt.Sprintf("%[1]*s", (70+len(msg))/2, msg))
}

func CreateRandomPwd(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

	byteRandomPwd := make([]byte, n)
	for i := range byteRandomPwd {
		byteRandomPwd[i] = letterBytes[rand.Int31n(int32(len(letterBytes)))]
	}
	return string(byteRandomPwd)
}

func GetUserFromSecret(path string) string {
	userByte, err := ioutil.ReadFile("/etc/mongodb-auth/username")
	if err != nil {
		glog.Fatalf("username err #%v", err)
	}
	return string(userByte)
}

func GetPwdFromSecret(path string) string {
	passByte, err := ioutil.ReadFile("/etc/mongodb-auth/password")
	if err != nil {
		glog.Fatalf("password err #%v", err)
	}
	return string(passByte)
}

func GetHostFromSecret(path string) string {
	userByte, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Fatalf("username err #%v", err)
	}
	return string(userByte)
}

func GetPortFromSecret(path string) string {
	userByte, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Fatalf("username err #%v", err)
	}
	return string(userByte)
}

func Register(name string, i broker.Interface) map[string]broker.Interface {
	mutex.Lock()
	defer mutex.Unlock()

	_, ok := LogicList[name]
	if ok {
		panic(fmt.Sprintf("duplicate register database %s", name))
	}

	LogicList[name] = i
	return LogicList
}

func GetList(name string) broker.Interface {
	return LogicList[name]
}

//GetMD5Password , password=MD5(instanceID+"Space")
func GetMD5Password(instanceID string) string {
	// genetrate encrypt password
	md5 := md5.New()
	io.WriteString(md5, instanceID+"spaceID")           //将instanceid写入到md5中
	userPasswordMD5 := hex.EncodeToString(md5.Sum(nil)) //md5.Sum(nil)将md5的hash转成[]byte格式
	fmt.Printf("MD5(userpasword)=%#v\n", userPasswordMD5)
	return userPasswordMD5
}

//GetConfig has admin right
func GetConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V2_2_0_0 //kafka版本号
	config.Net.SASL.Enable = true
	config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &xdg.XDGSCRAMClient{HashGeneratorFcn: scram.SHA256} }
	config.Net.SASL.Mechanism = "SCRAM-SHA-256"
	config.Net.SASL.User = "admin"
	config.Net.SASL.Password = "admin-secret"
	config.Consumer.Return.Errors = true
	// config.Net.SASL.Handshake = true
	// config.Producer.RequiredAcks = sarama.WaitForAll          // 发送完数据需要leader和follow都确认
	// config.Producer.Partitioner = sarama.NewRandomPartitioner // 新选出一个partition
	// config.Producer.Return.Successes = true                   // 成功交付的消息将在success channel返回
	return config
}

//kafka位置要改成從db抓 參數放brokers
func GetClusterAdmin(uri string) (sarama.ClusterAdmin, error) {
	kafkaBrokers := strings.Split(uri, ",")                         //sarama吃uri陣列                        //kafkaBrokers string= aaa:8000,bbb:8000 -> kafkaBrokers slice= [aaa:8000 bbb:8000]
	admin, err := sarama.NewClusterAdmin(kafkaBrokers, GetConfig()) //create a cluster admin
	//[]string{KafkaBrokers}
	if err != nil {
		log.Println("err: ", err)
		return nil, err
	}
	return admin, nil
}

//取得kafkashellIP
func GetKafkaShellIP(kafkashell string) string {
	var kafkaShellIP string
	// kafkaBrokers := strings.Split(uri, ",") //kafkaBrokers string= aaa:8000,bbb:8000 -> kafkaBrokers slice= [aaa:8000 bbb:8000]
	// if len(kafkaBrokers) != 0 {             //如果ip有值且正確
	// 	uri := strings.Split(kafkaBrokers[0], ":")
	// 	kafkaShellIP = "http://" + uri[0] + ":32000"
	// }
	kafkaShellIP = "http://" + kafkashell
	// fmt.Println("KafkaShellIP=", kafkaShellIP)
	return kafkaShellIP
}
