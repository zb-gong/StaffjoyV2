package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	// account_pkg "v2.staffjoy.com/account"
	// tracing "v2.staffjoy.com/tracing"
	// company_pkg "v2.staffjoy.com/company"
	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/metadata"
	"v2.staffjoy.com/auth"
	crypto "v2.staffjoy.com/crypto"
)

func phoneNumber() int {
	return 1000000000 + rand.Intn(8999999999)
}

type TeamCompanyPair struct {
	team_uuid    crypto.UUID
	company_uuid crypto.UUID
}

type Account struct {
	uuid        crypto.UUID
	name        string
	email       string
	phonenumber int
}

func generate_data() {
	fmt.Printf("Generating data\n")
	num_companies := 20
	num_teams := 100
	num_jobs := 100
	num_accounts := 100
	num_workers := 1000

	// var context context.Context
	context := context.TODO()
	context = metadata.AppendToOutgoingContext(context, auth.AuthorizationMetadata, auth.AuthorizationSupportUser)
	// // First generate companies
	// companyClient, close, err := company_pkg.NewClient("loadgen")
	// defer close()
	// check_err_debug(err, "Couldn't obtain a client to company")
	// accountClient, close, err := account_pkg.NewClient("loadgen")
	// defer close()
	// check_err_debug(err, "Couldn't obtain a client to account")

	db, err := sql.Open("mysql", "staffjoy:password@tcp(127.0.0.1:3306)/staffjoy?parseTime=true")

	_, _ = db.Exec("delete from account")
	_, _ = db.Exec("delete from admin")
	_, _ = db.Exec("delete from company")
	_, _ = db.Exec("delete from directory")
	_, _ = db.Exec("delete from job")
	_, _ = db.Exec("delete from manager")
	_, _ = db.Exec("delete from shift")
	_, _ = db.Exec("delete from staffjoy.admin")
	_, _ = db.Exec("delete from staffjoy.company")
	_, _ = db.Exec("delete from staffjoy.directory")
	_, _ = db.Exec("delete from staffjoy.job")
	_, _ = db.Exec("delete from staffjoy.manager")
	_, _ = db.Exec("delete from staffjoy.shift")
	_, _ = db.Exec("delete from staffjoy.team")
	_, _ = db.Exec("delete from staffjoy.worker")
	_, _ = db.Exec("delete from team")
	_, _ = db.Exec("delete from worker")

	companies := make([]*crypto.UUID, 0)
	teams := make([]TeamCompanyPair, 0)
	accounts := make([]Account, 0)

	log.Info("Creating companies")
	{
		sqlStr := "INSERT INTO company (uuid, name, archived, default_timezone, default_day_week_starts) values "
		vals := []interface{}{}
		for n := 0; n <= num_companies; n++ {
			sqlStr += "(?, ?, ?, ?, ?),"
			uuid, _ := crypto.NewUUID()
			vals = append(vals, uuid.String(), "company_"+strconv.Itoa(n), 0, "GMT", "Monday")
			companies = append(companies, uuid)
		}
		sqlStr = sqlStr[0 : len(sqlStr)-1]
		stmt, _ := db.Prepare(sqlStr)
		_, err = stmt.Exec(vals...)
		if err != nil {
			log.Error(err, "failed to insert companies in database")
		}
	}

	log.Info("Creating teams")
	{
		sqlStr := "INSERT INTO team (uuid, company_uuid, name, archived, timezone, day_week_starts, color) values "
		vals := []interface{}{}
		for n := 0; n <= num_teams; n++ {
			sqlStr += "(?, ?, ?, ?, ?, ?, ?),"
			uuid, _ := crypto.NewUUID()
			company_uuid := companies[rand.Intn(len(companies))]
			vals = append(vals, uuid.String(), company_uuid.String(), "team_"+strconv.Itoa(n),
				0, "GMT", "Monday", "FF0000")
			teams = append(teams, TeamCompanyPair{*uuid, *company_uuid})
		}
		sqlStr = sqlStr[0 : len(sqlStr)-1]
		stmt, _ := db.Prepare(sqlStr)
		_, err = stmt.Exec(vals...)
		if err != nil {
			log.Error(err, "failed to insert teams in database")
		}
	}

	// Create user accounts
	log.Info("Creating accounts")
	{
		sqlStr := "INSERT INTO account (uuid, email, name, phonenumber, member_since) values "
		vals := []interface{}{}
		for n := 0; n <= num_accounts; n++ {
			sqlStr += "(?, ?, ?, ?, ?),"
			uuid, _ := crypto.NewUUID()
			name := "account_" + strconv.Itoa(n)
			email := "account_em_" + strconv.Itoa(n) + "@server.com"
			phonenumber := phoneNumber()
			member_since := time.Now()
			// member_since := "2019-08-15 00:00:00.000"
			vals = append(vals, uuid.String(), email, name, strconv.Itoa(phonenumber), member_since)
			accounts = append(accounts, Account{*uuid, name, email, phonenumber})
		}
		sqlStr = sqlStr[0 : len(sqlStr)-1]
		stmt, _ := db.Prepare(sqlStr)
		_, err = stmt.Exec(vals...)
		if err != nil {
			log.Error(err, "failed to insert accounts in database")
		}
	}

	// For each company, add an admin
	log.Info("Adding admins")
	{
		dirSqlStr := "INSERT INTO directory (company_uuid, user_uuid, internal_id) values "
		admSqlStr := "INSERT INTO admin (company_uuid, user_uuid) values "
		dirVals := []interface{}{}
		admVals := []interface{}{}
		for n := 0; n <= num_companies; n++ {
			dirSqlStr += "(?, ?, ?),"
			admSqlStr += "(?, ?),"
			company_uuid := companies[n]
			user := accounts[rand.Intn(len(accounts))]
			internal_id := "iid_" + user.uuid.String()

			dirVals = append(dirVals, company_uuid.String(), user.uuid.String(), internal_id)
			admVals = append(admVals, company_uuid.String(), user.uuid.String())
		}
		dirSqlStr = dirSqlStr[0 : len(dirSqlStr)-1]
		dirStmt, _ := db.Prepare(dirSqlStr)
		_, err = dirStmt.Exec(dirVals...)
		if err != nil {
			log.Error(err, "failed to insert directories in database")
		}

		admSqlStr = admSqlStr[0 : len(admSqlStr)-1]
		admStmt, _ := db.Prepare(admSqlStr)
		_, err = admStmt.Exec(admVals...)
		if err != nil {
			log.Error(err, "failed to insert admins in database")
		}
	}

	log.Info("Creating jobs")
	{
		sqlStr := "INSERT INTO job (uuid, team_uuid, name, color) values "
		vals := []interface{}{}
		for n := 0; n <= num_jobs; n++ {
			sqlStr += "(?, ?, ?, ?),"
			uuid, _ := crypto.NewUUID()
			team := teams[rand.Intn(len(teams))]
			name := "job_" + strconv.Itoa(n)
			color := "000000"
			vals = append(vals, uuid.String(), team.team_uuid.String(), name, color)
		}
		sqlStr = sqlStr[0 : len(sqlStr)-1]
		stmt, _ := db.Prepare(sqlStr)
		_, err = stmt.Exec(vals...)
		if err != nil {
			log.Error(err, "failed to insert jobs in database")
		}
	}

	log.Info("Creating workers")
	{
		dirSqlStr := "INSERT INTO directory (company_uuid, user_uuid, internal_id) values "
		wrkSqlStr := "INSERT INTO worker (team_uuid, user_uuid) values "
		dirVals := []interface{}{}
		wrkVals := []interface{}{}
		for n := 0; n <= num_workers; n++ {
			team := teams[rand.Intn(len(teams))]
			account := accounts[rand.Intn(len(accounts))]
			internal_id := "iid_" + account.uuid.String()

			dirSqlStr += "(?, ?, ?),"
			wrkSqlStr += "(?, ?),"
			dirVals = append(dirVals, team.company_uuid.String(), account.uuid.String(), internal_id)
			wrkVals = append(wrkVals, team.team_uuid.String(), account.uuid.String())
		}
		dirSqlStr = dirSqlStr[0 : len(dirSqlStr)-1]
		dirStmt, _ := db.Prepare(dirSqlStr)
		_, err = dirStmt.Exec(dirVals...)
		if err != nil {
			log.Error(err, "failed to insert directories in database")
		}

		wrkSqlStr = wrkSqlStr[0 : len(wrkSqlStr)-1]
		wrkStmt, _ := db.Prepare(wrkSqlStr)
		_, err = wrkStmt.Exec(wrkVals...)
		if err != nil {
			log.Error(err, "failed to insert workers in database")
		}
	}
}

func main() {
	// _, closer := tracing.InitTracer("loadgen")
	// defer closer.Close()
	generate_data()
}
