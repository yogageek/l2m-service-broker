package shared

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "foo"

// Deprov determines enablement of deprovision feature
var Deprov bool = false

var (
	Description               = "MongoDB error"
	FailedEstablishConnection = "failed establish connection to MongoDB"
	CreateUserFailed          = "create user failed"
	DatabaseDeletionFailed    = "database deletion failed"
	CreateDatabaseFailed      = "create database failed"

	BindingFailed    = "Binding failed"
	UnbindFailed     = "Unbind failed"
	ProvisionFaied   = "Provision Faied"
	DeprovisionFaied = "Deprovision Faied"
	ConnectionFailed = "Connect Kafka Failed"
)

type mongodb struct {
	Ctx    context.Context
	Cli    *mongo.Client
	Cancel context.CancelFunc
}

func (m *mongodb) closed() error {
	if m.Cli == nil {
		return fmt.Errorf("client is nil")
	}
	return m.Cli.Disconnect(m.Ctx)
}

func newStart(uri string) (mongodb, error) {
	if !strings.HasPrefix(uri, "mongodb://") {
		uri = fmt.Sprintf("mongodb://%s", uri)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	cli, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		return mongodb{
			Cancel: cancel,
		}, err
	}
	err = cli.Connect(ctx)
	if err != nil {
		return mongodb{
			Cancel: cancel,
		}, err
	}

	return mongodb{Ctx: ctx, Cli: cli, Cancel: cancel}, nil
}

func (m *mongodb) createUser(databaseName, userName, password string) error {
	db := m.Cli.Database(databaseName)
	err := db.RunCommand(m.Ctx, bson.D{
		primitive.E{Key: "createUser", Value: userName},
		primitive.E{Key: "pwd", Value: password},
		primitive.E{Key: "roles", Value: bson.A{
			bson.D{primitive.E{Key: "role", Value: "readWrite"}, primitive.E{Key: "db", Value: databaseName}},
		}},
	}).Err()

	return err
}

func (m *mongodb) deleteUser(databaseName, userName string) error {
	db := m.Cli.Database(databaseName)

	err := db.RunCommand(m.Ctx, bson.D{primitive.E{Key: "dropUser", Value: userName}}, nil).Err()
	if err != nil {
		return err
	}

	db = m.Cli.Database("admin")
	_, err = db.Collection("system.users", nil).DeleteOne(m.Ctx, bson.D{primitive.E{Key: "user", Value: userName}}, nil)
	return err
}

func (m *mongodb) createDatabase(databaseName string) error {
	// Create database initiate directly by insert operation
	db := m.Cli.Database(databaseName)
	_, err := db.Collection(collectionName).InsertOne(m.Ctx, bson.M{"bar": "baz"}, &options.InsertOneOptions{})
	if err != nil {
		return err
	}

	// Drop collection after database created
	err = db.Collection(collectionName).Drop(m.Ctx)
	return err
}

func (m *mongodb) deleteDatabase(databaseName string) error {
	// Create database initiate directly by insert operation
	db := m.Cli.Database(databaseName)
	err := db.Drop(m.Ctx)
	if err != nil {
		return err
	}

	// Drop collection after database created
	err = db.Collection(collectionName).Drop(m.Ctx)
	return err
}

func nilString() string {
	return ""
}

func mapInstance(kafkaBrokers, topic, userID, userPassword string) map[string]interface{} {
	return map[string]interface{}{
		"kafkaBrokers": kafkaBrokers,
		"topic":        topic,
		"userID":       userID,
		"userPassword": userPassword,
	}
}

// 只有debug用
// For debugging purpose
func createURI() string {
	var cred string

	host1 := os.Getenv("MONGODB_HOST1")
	host2 := os.Getenv("MONGODB_HOST2")
	host3 := os.Getenv("MONGODB_HOST3")

	port1 := os.Getenv("MONGODB_PORT1")
	port2 := os.Getenv("MONGODB_PORT2")
	port3 := os.Getenv("MONGODB_PORT3")

	username := os.Getenv("MONGODB_USERNAME")
	password := os.Getenv("MONGODB_PASSWORD")

	if username != "" && password != "" {
		cred = fmt.Sprintf("%s:%s@", username, password)
	}

	return fmt.Sprintf("mongodb://%s%s:%s,%s:%s,%s:%s", cred, host1, port1, host2, port2, host3, port3)
}

func checkConnection() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uri := createURI()
	cli, _ := mongo.NewClient(options.Client().ApplyURI(uri))

	err := cli.Connect(ctx)
	if err != nil {
		glog.Errorf("checkConnection failed %+v", err)
	}
	cli.Disconnect(ctx)

	return true
}
