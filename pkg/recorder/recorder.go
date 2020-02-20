package recorder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"

	_ "github.com/lib/pq"
)

var (
	pgDriver = "postgres"

	opshost1       = os.Getenv("OPS_PG_HOST1")
	opsport1       = os.Getenv("OPS_PG_PORT1")
	opsusername    = os.Getenv("OPS_PG_USERNAME")
	opspassword    = os.Getenv("OPS_PG_PASSWORD")
	datacenterCode = os.Getenv("DATACENTER_CODE")
)

const (
	// InstanceIDInUse declared if requested instance ID is exist in ops database.
	InstanceIDInUse = "instance_id in use"

	// InstanceIDNotFound declared if requested instance ID is exist in ops database.
	InstanceIDNotFound = "instance_id not found"
)

// CheckConnection do check connection on ops database before all process begin
func CheckConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	glog.V(2).Info("Checking database connection ...")
	uri := CreateURI(databaseName) //databaseName=kafka
	// fmt.Println("kafka database uri  location:", uri)
	db, err := sql.Open("postgres", uri)
	if err != nil {
		glog.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		if strings.Contains(err.Error(), "connect: connection refused") {
			retryConnection(db)
		} else {
			glog.Fatal(err)
		}
	}
	return nil
}

// if  pg ops db can't connect at first time, retry a few times
func retryConnection(db *sql.DB) {
	timer := time.NewTimer(time.Second * 30)
	done := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-timer.C:
				glog.Fatal("failed to establish connection")
			case <-done:
				return
			}
		}
	}()

	var i int
	err := db.Ping()
	for err != nil {
		i++

		time.Sleep(time.Second * 5)
		glog.Errorf("%s, reconnecting ... [%d]", err.Error(), i)
		err = db.Ping()
	}
	done <- true
}

// #SM1-003 Deprecated create database on start
//
// func initDatabase() {
// 	// since this is the first initiation, need to use postgres account
// 	uri := CreateURI("postgres")
// 	db, _ := sql.Open(pgDriver, uri)
// 	defer db.Close()
// 	glog.Warning("creating database ...")
// 	createDatabase(db)
// }

// InitRecorder does init on recorder
//if PG credential table exist, but can't find  the env (datacenterCode) -> add one to credential table
//if PG credential table exist and have find the env(datacenterCode) -> ignore
//if PG credential table  not exist -> init all tables and add one to credential table
func InitRecorder() error {
	uri := CreateURI(databaseName)
	db, err := sql.Open(pgDriver, uri)
	if err == nil {
		var check bool
		err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM credentials where env='" + datacenterCode + "')").Scan(&check) //if err, check default value =false
		// whcih means  credential table exist, because if not , will get err
		if err == nil {
			fmt.Println("if datacenterCode already exist in ops db: ", check) //if no err, check=true/false
			if !check {
				fmt.Println("datacenterCode not exist in ops db, add one to credential table")
				initStages(db)
			}
		}
		// if PG credential table not exist, will get err->pq: relation "credentials" does not exist
		//如果kafkadb  has no table()=crendential)，首次啟動會->pq: relation "credentials" does not exist
		// whcih means  credential table not exist, but may also be other errs!
		if err != nil {
			fmt.Printf("InitRecorder err:%s. Init all tables to ops db", err)
			glog.Info("initiate table creation")
			err = initStages(db)
			if err != nil {
				glog.Error(err)
			}
			return err
		}
	}
	return nil
}

// Recorder struct
type Recorder struct {
	Cli *sql.DB
}

// seems not using
type cred struct {
	pid     string
	shared  bool
	uri     string
	user    string
	pass    string
	dbAuthz string
}

type mapBuffer struct {
	uri   string
	dbPid string
}

// New initiate connection
func New() *Recorder {
	uri := CreateURI(databaseName) //databaseName is declared in handler.go var because they are in the same package
	db, err := sql.Open(pgDriver, uri)
	if err != nil {
		glog.Error(err)
	}
	if err := db.Ping(); err != nil {
		glog.Error(err)
	}
	return &Recorder{Cli: db}
}

// OnProvision validates and record all Provision ops log then store into ops db
func (r *Recorder) OnProvision(fn ObjectProvision) ObjectProvision {
	return func(request *osb.ProvisionRequest, c *broker.RequestContext) (*broker.ProvisionResponse, error) {
		//id, status, spaceID就是查出來的instance_id, instance_status, spaces_id
		var id, status, spaceID string
		errc := make(chan error)
		// 去 share_instance 查看 instance_id 是否已存在
		// Check instance_id and instance_status to see if instance already exists
		if e := r.Cli.QueryRow(`SELECT instance_id, instance_status, spaces_id FROM share_instance where instance_id = $1`,
			request.InstanceID).Scan(&id, &status, &spaceID); e != nil { //&id, &status, &spaceID就是查出來的instance_id, instance_status, spaces_id
			// yoga
			// fmt.Println("OnProvision id, status, spaceID:", id, status, spaceID)
			if e != sql.ErrNoRows {
				errString := errToString(e)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		}

		if id == request.InstanceID && (spaceID != request.SpaceGUID || status != statusDeleted) {
			errString := errToString(fmt.Errorf(InstanceIDInUse))
			desc := fmt.Sprintf("instance_id: %s; space_id: %s", request.InstanceID, request.SpaceGUID)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: &errString,
				Description:  &desc,
			}
		}

		var err error
		mb := &mapBuffer{}

		var dc string
		var ok bool
		// #11175
		tmp := request.Parameters["datacenterCode"]
		if dc, ok = tmp.(string); !ok {
			errString := http.StatusText(http.StatusBadRequest)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusBadRequest,
				ErrorMessage: &errString}
		}

		// get target database
		// fmt.Printf("%+v\n", request)
		//fmt.Println("datacenterCode:", env)
		//fmt.Println("id:", id)//id=instance_id
		// fmt.Println("maxInstance:", maxInstance)

		mb, err = r.getTargetDatabase(dc, id, maxInstance, true)
		if err != nil {
			errString := fmt.Sprintf("getting target database from datacenterCode: '%s'; failed; %+v", dc, err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusInsufficientStorage,
				ErrorMessage: &errString,
			}
		}

		// Pass values to be callable by logic business to connect to target database based on information
		// from ops db
		//当<- chan 的时候是对chan中的数据读取；
		//相反 chan <- value 是对chan赋值。
		go func() {
			err := pubMsg(&mb.uri)
			if err != nil {
				errc <- err
				glog.Error(err)
			}
		}()

		// Logic business function
		res, err := fn(request, c)
		// fmt.Printf("%+v\n: %+v\n:", request, c)

		// This section will not handle any error, allowed to print to stderr but should pass error without mutate.
		if err != nil {
			glog.Error(err) //會印紅字
			return nil, err
		}

		tx, err := r.Cli.Begin()
		defer txRollback(tx, err)

		if err != nil {
			errString := errToString(err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusServiceUnavailable,
				ErrorMessage: &errString,
			}
		}

		// modify Bug #11795 當depro後重新pro,要把delete instance time 改為null
		// create a sql.NullString
		var s string
		nullString := &s
		if s == "" {
			nullString = nil
		}
		// No checking on request instance and status since not exist
		if id == request.InstanceID && status == statusDeleted {
			_, err := tx.Exec(`UPDATE share_instance SET instance_status = $1, delete_instance_time = $2 
	where instance_id = $3`, statusExist, nullString, request.InstanceID)

			// Confirm rows get updated. Return response Exists:true when at least 1 row get updated.
			if err != nil {
				errString := errToString(err)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}

		} else {
			// Not set instance_parameters
			_, err = tx.Exec(`INSERT INTO share_instance (database_pid, instance_id, plan_id, plan_name, org_id, spaces_id,
	create_instance_time,instance_status) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				mb.dbPid, request.InstanceID, request.PlanID, planName, request.OrganizationGUID, request.SpaceGUID, time.Now(), statusExist)
			if err != nil {
				errString := errToString(err)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		}
		//The select statement lets a goroutine wait on multiple communication operations.
		//A select blocks until one of its cases can run, then it executes that case. It chooses one at random if multiple are ready.
		select {
		case <-errc:
			return nil, <-errc
		default:
			return res, nil
		}
	}
}

// OnDeprovision validates and record all Deprovision ops log then store into ops db
func (r *Recorder) OnDeprovision(fn ObjectDeprovision) ObjectDeprovision {
	return func(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {

		var id, status, instancePID string
		errc := make(chan error)
		// Check to see if instance already exists
		r.Cli.QueryRow(`SELECT share_instance_pid, instance_id, instance_status FROM share_instance where instance_id = $1`,
			request.InstanceID).Scan(&instancePID, &id, &status)
		if status == statusRemoved {
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: &statusRemoved,
			}
		}
		// # 11145 & #11158
		if (id == request.InstanceID && status == statusExist) == false {
			errString := errToString(fmt.Errorf(InstanceIDNotFound))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: &errString,
			}
		}
		//yoga
		// Check to see if instance already exists (isn't is shoud be check if haven't unbind?)
		var pidStatus string
		noDataOrErr := r.Cli.QueryRow(`SELECT binding_status FROM share_binding where share_instance_pid = $1`,
			instancePID).Scan(&pidStatus)
		if noDataOrErr == nil {
			if pidStatus != statusDeleted {
				errString := errToString(fmt.Errorf("can't deprovision before unbind"))
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusBadRequest,
					ErrorMessage: &errString,
				}
			}
		} else {
			glog.Info("err:", noDataOrErr)
		}
		// bug (if never bind, can't deprovision after provision)
		// // Check to see if instance already exists (isn't is shoud be check if haven't unbind?)
		// var pidStatus string
		// r.Cli.QueryRow(``, instancePID).Scan(&pidStatus)
		// if pidStatus != statusDeleted {
		// 	errString := http.StatusText(http.StatusBadRequest)
		// 	return nil, osb.HTTPStatusCodeError{
		// 		StatusCode:   http.StatusBadRequest,
		// 		ErrorMessage: &errString,
		// 	}
		// }

		var err error
		mb := &mapBuffer{}
		mb, err = r.getTargetDatabase("", id, maxInstance, false)
		if err != nil {
			errString := fmt.Sprintf("getting target database from id: %s failed; %+v", id, err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusInsufficientStorage,
				ErrorMessage: &errString,
			}
		}

		// Pass values to be callable by logic business to connect to target database based on information
		// from ops db
		go func() {
			err := pubMsg(&mb.uri)
			if err != nil {
				errc <- err
				glog.Error(err)
			}
		}()

		res, err := fn(request, c)

		// This section will not handle any error, allowed to print to stderr but should pass error without mutate.
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		tx, err := r.Cli.Begin()
		defer txRollback(tx, err)

		if err != nil {
			errString := errToString(err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusServiceUnavailable,
				ErrorMessage: &errString,
			}
		}

		if id == request.InstanceID && status == statusExist {

			// Update rows
			_, err := r.Cli.Exec(`UPDATE share_instance set instance_status=$1, delete_instance_time=$2 where instance_id=$3`,
				statusDeleted, time.Now(), request.InstanceID)

			// Confirm rows get updated. Return response Exists:true when at least 1 row get updated.
			if err != nil {
				errString := errToString(err)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		}

		select {
		case <-errc:
			return nil, <-errc
		default:
			return res, nil
		}
	}
}

// OnBind validates and record all Bind ops log then store into ops db
func (r *Recorder) OnBind(fn ObjectBind) ObjectBind {
	return func(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {

		var id, status string
		var pid *string
		var bindID, bindStatus string
		var bindQuantity int
		errc := make(chan error)

		// Check to see if instance already exists
		r.Cli.QueryRow(`SELECT share_instance_pid, instance_id, instance_status FROM share_instance where instance_id = $1`,
			request.InstanceID).Scan(&pid, &id, &status)

		if !(id == request.InstanceID && status == statusExist) {
			errString := errToString(fmt.Errorf(InstanceIDNotFound))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: &errString,
			}
		}

		// Check to see if bind already exists
		r.Cli.QueryRow(`SELECT binding_id, binding_status FROM share_binding where binding_id = $1`,
			request.BindingID).Scan(&bindID, &bindStatus)
		if bindID == request.BindingID && bindStatus == statusExist {
			errString := errToString(fmt.Errorf("binding_id in use"))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: &errString,
			}
		}

		//limit bind quantity-yoga
		//select COUNT(binding_id) FROM public.share_binding where share_instance_pid=14 and binding_status='exist'
		r.Cli.QueryRow(`SELECT COUNT(binding_id) FROM share_binding WHERE share_instance_pid= $1 and binding_status='exist'`,
			pid).Scan(&bindQuantity)
		maxbind, _ := strconv.Atoi(os.Getenv("MAX_BIND_PER_INSTANCE"))
		if bindQuantity >= maxbind {
			errString := errToString(fmt.Errorf("binding_id already reached the limit "))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: &errString,
			}
		}

		var err error
		mb := &mapBuffer{}

		mb, err = r.getTargetDatabase("", id, maxInstance, false)
		if err != nil {
			errString := fmt.Sprintf("getting target database from id: %s failed; %+v", id, err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusInsufficientStorage,
				ErrorMessage: &errString,
			}
		}

		// Pass values to be callable by logic business to connect to target database based on information
		// from ops db
		go func() {
			err := pubMsg(&mb.uri)
			if err != nil {
				errc <- err
				glog.Error(err)
			}
		}()

		res, err := fn(request, c)

		// This section will not handle any error, allowed to print to stderr but should pass error without mutate.
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		tx, err := r.Cli.Begin()
		defer txRollback(tx, err)

		if bindID == request.BindingID && bindStatus == statusDeleted {

			// Update rows
			_, err := r.Cli.Exec(`UPDATE share_binding set binding_status=$1, binding_time=$2 where binding_id=$3`,
				statusExist, time.Now(), request.BindingID)

			// Confirm rows get updated. Return response Exists:true when at least 1 row get updated.
			if err != nil {
				errString := errToString(err)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		} else {
			// Not set instance_parameters
			_, err := r.Cli.Exec(`INSERT INTO share_binding (share_instance_pid, binding_id, binding_time, binding_status) VALUES($1, $2, $3, $4)`,
				pid, request.BindingID, time.Now(), statusExist)
			if err != nil {
				errString := errToString(fmt.Errorf(InstanceIDNotFound))
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		}

		select {
		case <-errc:
			return nil, <-errc
		default:
			return res, nil
		}
	}
}

// OnUnbind validates and record all Unbind ops log then store into ops db
func (r *Recorder) OnUnbind(fn ObjectUnbind) ObjectUnbind {
	return func(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {

		var id, status string
		var bindID, bindStatus string
		var belong bool
		errc := make(chan error)

		// Check to see if instance already exists
		r.Cli.QueryRow(`SELECT instance_id, instance_status FROM share_instance where instance_id = $1`,
			request.InstanceID).Scan(&id, &status)

		if !(id == request.InstanceID && status == statusExist) {
			errString := fmt.Sprintf(InstanceIDNotFound)
			glog.Error(errString)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: &errString,
				Description:  &request.InstanceID,
			}
		}

		// Check to see if bind already exists
		r.Cli.QueryRow(`SELECT binding_id, binding_status FROM share_binding where binding_id = $1`,
			request.BindingID).Scan(&bindID, &bindStatus)

		// v0.1.1 spec:
		// duplicated unbind return 404
		if bindID == request.BindingID && bindStatus == statusDeleted {
			errString := errToString(fmt.Errorf("Already Unbinded"))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: &errString,
				Description:  &request.BindingID,
			}
		}

		//1.4.1 spec  can not unbind binding_id which is not belong to instance_id
		//select share_instance_pid from share_binding where binding_id='vmbid' and binding_status='exist'
		r.Cli.QueryRow(`SELECT EXISTS (SELECT 1	FROM share_binding, share_instance
			WHERE share_binding.share_instance_pid = share_instance.share_instance_pid AND share_instance.instance_id=$1 AND share_binding.binding_id=$2)`,
			request.InstanceID, request.BindingID).Scan(&belong)
		if !belong {
			errString := errToString(fmt.Errorf("binding_id not belong to instance_id or binding_id not exist"))
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: &errString,
				Description:  &request.BindingID,
			}
		}

		var err error
		mb := &mapBuffer{}

		mb, err = r.getTargetDatabase("", id, maxInstance, false)
		if err != nil {
			errString := fmt.Sprintf("getting target database from id: %s failed; %v", id, err)
			return nil, osb.HTTPStatusCodeError{
				StatusCode:   http.StatusInsufficientStorage,
				ErrorMessage: &errString,
			}
		}

		// Pass values to be callable by logic business to connect to target database based on information
		// from ops db
		go func() {
			err := pubMsg(&mb.uri)
			if err != nil {
				errc <- err
				glog.Error(err)
			}
		}()

		res, err := fn(request, c)

		// This section will not handle any error, allowed to print to stderr but should pass error without mutate.
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		tx, err := r.Cli.Begin()
		defer txRollback(tx, err)

		if bindID == request.BindingID && bindStatus == statusExist {
			_, err := r.Cli.Exec(`UPDATE share_binding set binding_status=$1, unbinding_time=$2 where binding_id=$3`,
				statusDeleted, time.Now(), request.BindingID)
			if err != nil {
				errString := errToString(err)
				return nil, osb.HTTPStatusCodeError{
					StatusCode:   http.StatusServiceUnavailable,
					ErrorMessage: &errString,
				}
			}
		}

		select {
		case <-errc:
			return nil, <-errc
		default:
			return res, nil
		}
	}
}

func txRollback(tx *sql.Tx, err error) {
	switch err {
	case nil:
		err = tx.Commit()
	default:
		tx.Rollback()
	}
}

type credential struct {
	dbPid    string
	uri      string
	username string
	password string
}

// getTargetDatabase is part of #SM1-004
func (r Recorder) getTargetDatabase(dc, instanceID string, lim int, provision bool) (*mapBuffer, error) {
	// check if there is datacenterCode value of not
	var err error
	mb := &mapBuffer{}
	// if not, query database pid from share_instance where database_pid=$1
	switch provision {
	case true:
		mb, err = r.getTargetDatabaseByDatacenterCode(dc, lim)
	default:
		mb, err = r.getTargetDatabaseByInstanceID(instanceID, lim)
	}
	// return uri
	return mb, err
}

// getTargetDatabaseByInstanceID is part of #SM1-004
func (r Recorder) getTargetDatabaseByInstanceID(instanceID string, lim int) (*mapBuffer, error) {
	mb := mapBuffer{}
	// if not, query database pid from share_instance where database_pid=$1
	r.Cli.QueryRow(`SELECT database_pid FROM share_instance where instance_id=$1`, instanceID).Scan(&mb.dbPid)

	// then query uri from credentials where database_pid=$1
	r.Cli.QueryRow(`SELECT uri FROM credentials where database_pid=$1`, mb.dbPid).Scan(&mb.uri)

	return &mb, nil
}

// getTargetDatabaseByDatacenterCode is part of #SM1-004
func (r Recorder) getTargetDatabaseByDatacenterCode(dc string, lim int) (*mapBuffer, error) {
	var datacenterCodeQuery string
	// res is temporary buffer
	res := mapBuffer{}

	results := map[string]string{}

	// if datacenterCode is empty or contains whitespace
	if dc == "" || strings.ContainsAny(dc, " ") {
		// datacenterCode can't be NULL
		// datacenterCodeQuery = "datacenter_code is NULL"
		return nil, errors.New("datacenterCode contains illegal character")
	}
	// yoga modify
	datacenterCodeQuery = fmt.Sprintf("env='%s'", dc)
	// query database_pid based on datacenterCode
	// yoga modify
	s, err := r.Cli.Query(fmt.Sprintf(`SELECT database_pid, uri FROM credentials where %s`, datacenterCodeQuery))
	if err != nil {
		return nil, err
	}

	// store to map[db_pid]
	for s.Next() {
		s.Scan(&res.dbPid, &res.uri)
		results[res.dbPid] = res.uri
	}

	//1.4.1 - check if datacenterCode exist in ops db
	if _, ok := results[res.dbPid]; !ok {
		return nil, errors.New("invalid datacenterCode")
	}

	// remove empty key-value pair on map if any
	delete(results, "")
	// this variable will stores 'k' value when for loop ended
	var keyPid string

	for k := range results {
		if ok := r.checkOccupancy(k, lim); !ok {
			fmt.Printf("co %+v more than limited instanceID: %d", results, lim)
			glog.Infof("co %s more than lim: %d", k, lim)
			delete(results, k)
			continue
		}
		keyPid = k
	}

	// if map length is 0, means all shared host db occupied, return 507
	if len(results) == 0 {
		return nil, fmt.Errorf(http.StatusText(http.StatusInsufficientStorage))
	}

	mb := &mapBuffer{
		dbPid: keyPid,
		uri:   results[keyPid],
	}

	// return map string contains 1 or more target db uri and pid
	return mb, nil
}

// checkOccupancy checks whether DB is occupied. return false if DB is occupied.  occupied means instance already reach to the limit
func (r Recorder) checkOccupancy(dbPid string, lim int) bool {
	var count int
	// #11124
	err := r.Cli.QueryRow(`SELECT count(instance_id) FROM share_instance WHERE database_pid=$1 AND instance_status!=$2`, dbPid, statusRemoved).Scan(&count)
	if err != nil {
		return false
	}
	if count >= lim {
		return false
	}
	return true
}

// not sure
func CreateURI(dbAuthz string) string {
	var cred string

	if _, err := strconv.Atoi(opsport1); err != nil {
		glog.Fatalf("invalid port: %s", opsport1)
	}

	if opsusername != "" && opspassword != "" {
		cred = fmt.Sprintf("%s:%s@", opsusername, opspassword)
	}
	// #SM1-005 Service Manager dies when OPS database does not exist => impact: hardcoded database name to postgresql url
	return fmt.Sprintf("postgres://%s%s:%s/%s?sslmode=disable", cred, opshost1, opsport1, databaseName)
}

func errToString(e error) string {
	glog.Error(e)
	return e.Error()
}
