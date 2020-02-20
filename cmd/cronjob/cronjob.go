// #SM1-001 Add job to delete expired database

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"time"

	"github.com/golang/glog"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	maxDays  int
	host1    string
	port1    string
	dbAuthz  string
	username string
	password string
)

func init() {
	flag.IntVar(&maxDays, "maxperiod", 1, "MAX_DAYS_PREDELETION_PERIOD")
	flag.StringVar(&host1, "host", "localhost", "Ops database host")
	flag.StringVar(&port1, "port", "5432", "Ops database port")
	flag.StringVar(&username, "username", "postgres", "Ops database username")
	flag.StringVar(&password, "password", "", "Ops database password")
	flag.StringVar(&dbAuthz, "dbauth", "", "Ops database authentication db")
	flag.Parse()
}

func preDeletion() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cred := fmt.Sprintf("%s:%s@", username, password)
	uri := fmt.Sprintf("postgres://%s%s:%s/%s?sslmode=disable", cred, host1, port1, dbAuthz)
	db, err := sql.Open("postgres", uri)
	if err != nil {
		return err
	}

	// check connection
	if err = db.PingContext(ctx); err != nil {
		return err
	}

	result, err := db.Query(fmt.Sprintf(`SELECT database_pid, instance_id, delete_instance_time FROM share_instance WHERE delete_instance_time < now() - interval '%d minutes' AND delete_instance_time is not null`, maxDays))
	defer result.Close()
	if err != nil {
		return err
	}

	for result.Next() {
		var databasePid, instanceID, uri string
		var deleteTS time.Time
		// query database pid as a query key for credential table, store instance id and delete ts
		// for further process
		err := result.Scan(&databasePid, &instanceID, &deleteTS)
		if err != nil {
			return err
		}

		// query uri to connect to target database, uri already includes username, password and dbauth
		err = db.QueryRow(`SELECT uri FROM credentials WHERE database_pid=$1`, databasePid).Scan(&uri)
		if err != nil {
			return err
		}

		// initiate connection on target database
		ctxMongo, cancel := context.WithCancel(context.Background())
		defer cancel()

		cli, _ := mongo.NewClient(options.Client().ApplyURI(uri))
		err = cli.Connect(ctxMongo)
		if err != nil {
			return err
		}
		// sleep period
		glog.Infof("begin removal process database_pid: %s; instance_id: %s; ts: %v;, duration: %v;", databasePid, instanceID, deleteTS, time.Since(deleteTS))
		glog.Warning("initiate removal process")
		time.Sleep(5 * time.Second)

		err = cli.Database(instanceID).Drop(ctxMongo)
		if err != nil {
			return err
		}

		// check db existence
		listNames, _ := cli.Database(instanceID).ListCollectionNames(ctxMongo, nil, options.ListCollections())
		if len(listNames) != 0 {
			return fmt.Errorf("db still exist, failed to delete")

		}

		glog.Info("cleaning up")

		// #SM1-002 Add status "removed" to share_instance table represent expired database has been removed.
		// #11123
		if _, err := db.Exec(`UPDATE share_instance SET instance_status = $1 where instance_id = $2`, "removed", instanceID); err != nil {
			return err
		}
		glog.Info("removal process complete")
	}

	return nil
}

func main() {
	glog.Info("running job")
	err := preDeletion()
	if err != nil {
		glog.Error(err)
		return
	}

	glog.Info("close the job")
}

// I1224 09:13:50.263936       1 cronjob.go:104] running job
// panic: runtime error: invalid memory address or nil pointer dereference
// [signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x4f1fb2]

// goroutine 1 [running]:
// database/sql.(*Rows).close(0x0, 0x0, 0x0, 0x0, 0x0)
// 	/snap/go/4901/src/database/sql/sql.go:3063 +0x72
// database/sql.(*Rows).Close(0x0, 0x44, 0x0)
// 	/snap/go/4901/src/database/sql/sql.go:3059 +0x33
// main.preDeletion(0xa58ea0, 0xc000184480)
// 	/home/bee/Code/p/go/src/advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/cmd/cronjob/cronjob.go:46 +0xe15
// main.main()
// 	/home/bee/Code/p/go/src/advgitlab.eastasia.cloudapp.azure.com/pcf-resource/wise-paas-service-manager-mongodb/cmd/cronjob/cronjob.go:105 +0x84
