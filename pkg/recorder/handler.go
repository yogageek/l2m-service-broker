package recorder

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//handler.go are all about create tables in ops db
var (
	planName = "Shared"
	//this var will affect a lots func related to recorder
	// databaseName = "kafka"
	databaseName = os.Getenv("OPS_DATABASE_NAME")

	statusDeleted = "delete"
	statusExist   = "exist"
	statusRemoved = "removed"

	stages = map[string]string{} //stages stores each create table string for different table in map[string]string format
	_      = syntaxPq()          //this means initiate the function when starting declareing the vars. Assign func to as an _ because name is not used

	maxInstance, _ = strconv.Atoi(os.Getenv("MAX_INSTANCE_PER_DB"))
	pseudoID       = os.Getenv("PSEUDO_ID")
	shared         = os.Getenv("SHARED_USE")
	datacentercode = os.Getenv("DATACENTER_CODE")
	uri            = os.Getenv("LWM_IP")
)

func syntaxPq() map[string]string {

	// #SM1-003 Deprecated create database on start
	//
	// stages["database"] = fmt.Sprintf("CREATE DATABASE %s;", databaseName)

	stages["credentials"] = fmt.Sprint(`CREATE TABLE IF NOT EXISTS credentials (
		database_pid 			SERIAL  PRIMARY KEY		NOT NULL,
		pseudo_id	 			VARCHAR(50) 			NOT NULL,
		shared_use 				BOOLEAN 				NOT NULL,
		uri 					          VARCHAR(500),	
		env 					VARCHAR(50),
		status					VARCHAR(50)	
		)`)

	stages["ip"] = fmt.Sprint(`CREATE TABLE IF NOT EXISTS ip (
			ip_id 					SERIAL  PRIMARY KEY								NOT NULL,
			database_pid	 		SERIAL REFERENCES credentials(database_pid) 	NOT NULL,
			internal_ip 			INET	 										NOT NULL,
			public_ip 				INET,		
			port 					INTEGER,	
			in_use 					BOOLEAN	
			)`)

	stages["usage"] = fmt.Sprint(`CREATE TABLE IF NOT EXISTS usage (
				usage_pid	 			SERIAL  PRIMARY KEY							NOT NULL,
				database_pid			SERIAL REFERENCES credentials(database_pid)	NOT NULL,
				cpu 					VARCHAR(16)									NOT NULL,
				memory 					VARCHAR(16)									NOT NULL,
				usage_calculate			VARCHAR(16)									NOT NULL
				)`)

	stages["shareInstance"] = fmt.Sprint(`CREATE TABLE IF NOT EXISTS share_instance (
					share_instance_pid 		SERIAL  PRIMARY KEY 						NOT NULL,
					database_pid			SERIAL REFERENCES credentials(database_pid)	NOT NULL,
					instance_id 			VARCHAR(50) 								NOT NULL,
					plan_id 				VARCHAR(50) 								NOT NULL,
					plan_name 				VARCHAR(50) 								NOT NULL,
					org_id 					VARCHAR(50) 								NOT NULL,
					spaces_id 				VARCHAR(50)									NOT NULL,
					create_instance_time 	timestamptz,
					delete_instance_time 	timestamptz,
					instance_status			VARCHAR(16),
					instance_parameters		VARCHAR(256) 
				)`)
	stages["shareBinding"] = fmt.Sprint(`CREATE TABLE IF NOT EXISTS share_binding (
					share_binding_pid 		SERIAL  PRIMARY KEY										NOT NULL,
					share_instance_pid 		SERIAL REFERENCES share_instance(share_instance_pid)	NOT NULL,
					binding_id 				VARCHAR(50) 											NOT NULL,
					binding_time			timestamptz 	 										NOT NULL,
					binding_status			VARCHAR(16),
					unbinding_time 			timestamptz, 		
					binding_parameters		VARCHAR(256) 			
				)`)

	return stages //return just a map[string]string! (actually its just a "create table string for db")
}

//啟動SB時創建tables may get credential exist / no schema has been selected to create in
func initStages(db *sql.DB) error {
	sortStages := []string{"credentials", "ip", "usage", "shareInstance", "shareBinding"}
	for _, v := range sortStages {
		if _, err := db.Exec(stages[v]); err != nil { //loop all sortStages values to get key-value from stage(=map[string]string) to db.Exec
			// fmt.Println(err)
			return err
		}
	}

	if err := initInsertion(db); err != nil {
		return err
	}
	return nil
}

//啟動SB時插入kafka資訊到credential Table
func initInsertion(db *sql.DB) error {
	//啟動時插入資料
	if _, err := db.Exec(fmt.Sprint(`INSERT INTO credentials (pseudo_id, shared_use, uri, env) VALUES ($1, $2, $3, $4)`),
		pseudoID, shared, uri, datacentercode); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

//比較傳進來的string是否相同(依照順序倆倆比較) 檢查參數1,2,3是否有重複 不重複回true
func stringValidator(str ...string) bool {
	for i := 0; i < len(str)-1; i++ {
		if strings.Compare(str[i], str[i+1]) != 0 {
			return true
		}
	}
	return false
}
