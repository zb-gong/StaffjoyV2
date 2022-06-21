package main

import (
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pb2 "v2.staffjoy.com/account"
	pb "v2.staffjoy.com/company"
	"v2.staffjoy.com/environments"
	pb3 "v2.staffjoy.com/frontcache"
	"v2.staffjoy.com/healthcheck"
)

const (
	ServiceName = "frontcacheserver"
)

var (
	logger *logrus.Entry
	config environments.Config
)

type frontcacheServer struct {
	logger      *logrus.Entry
	errorClient environments.SentryClient

	use_caching  bool
	use_callback bool
	// ListWorkers cache & get_workers_in_team cache; key is teamuuid
	workers_cache map[string]*pb.Workers
	workers_lock  sync.RWMutex
	// ListJobs cache; key is teamuuid
	jobs_cache map[string]*pb.JobList
	jobs_lock  sync.RWMutex
	// GetJob cache; key is jobuuid
	job_cache map[string]*pb.Job
	job_lock  sync.RWMutex
	// GetCompany cache; key is companyuuid
	company_cache map[string]*pb.Company
	company_lock  sync.RWMutex
	// ListTeams cache; key is companyuuid
	teams_cache map[string]*pb.TeamList
	teams_lock  sync.RWMutex
	// GetTeams cache; key is teamuuid
	team_cache map[string]*pb.Team
	team_lock  sync.RWMutex
	// Listadmin cache; key is companyuuid
	admins_cache map[string]*pb.Admins
	admins_lock  sync.RWMutex
	// GetWorkerTeamInfo cache; key is workeruuid
	workerteam_cache map[string]*pb.Worker
	workerteam_lock  sync.RWMutex
	// account cache; key is accountuuid
	account_cache map[string]*pb2.Account
	account_lock  sync.RWMutex
}

func init() {
	var err error
	config, err = environments.GetConfig(os.Getenv(environments.EnvVar))
	if err != nil {
		panic("Unable to determine frontcacheserver configuration")
	}
	logger = config.GetLogger(ServiceName)
}

func main() {
	logger.Debugf("Booting frontcacheserver environment %s", config.Name)

	s := &frontcacheServer{logger: logger}
	if !config.Debug {
		s.errorClient = environments.ErrorClient(&config)
	}

	s.use_caching = (os.Getenv("USE_CACHING") == "1")
	s.use_callback = (os.Getenv("USE_CALLBACK") == "1")
	if s.use_caching {
		s.workers_cache = make(map[string]*pb.Workers)
		s.jobs_cache = make(map[string]*pb.JobList)
		s.job_cache = make(map[string]*pb.Job)
		s.company_cache = make(map[string]*pb.Company)
		s.teams_cache = make(map[string]*pb.TeamList)
		s.team_cache = make(map[string]*pb.Team)
		s.admins_cache = make(map[string]*pb.Admins)
		s.workerteam_cache = make(map[string]*pb.Worker)
		s.account_cache = make(map[string]*pb2.Account)
	}

	lis, err := net.Listen("tcp", pb3.ServerPort)
	if err != nil {
		logger.Panicf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb3.RegisterFrontCacheServiceServer(grpcServer, s)

	// set up a health check listener for kubernetes
	go func() {
		logger.Debugf("Booting frontcacheserver health check %s", config.Name)
		http.HandleFunc(healthcheck.HEALTHPATH, healthcheck.Handler)
		http.ListenAndServe(":9876", nil)
	}()

	s.logger.Infof("Starting to listen frontcacheserver service")
	grpcServer.Serve(lis)
}
