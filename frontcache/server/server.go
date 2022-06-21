package main

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"v2.staffjoy.com/account"
	"v2.staffjoy.com/company"
	"v2.staffjoy.com/frontcache"
	"v2.staffjoy.com/helpers"
)

func (s *frontcacheServer) internalError(err error, format string, a ...interface{}) error {
	s.logger.Errorf("%s: %v", format, err)
	if s.errorClient != nil {
		s.errorClient.CaptureError(err, nil)
	}
	return grpc.Errorf(codes.Unknown, format, a...)
}

func copyAccountToDirectory(a *account.Account, d *company.DirectoryEntry) {
	d.UserUuid = a.Uuid
	d.Name = a.Name
	d.ConfirmedAndActive = a.ConfirmedAndActive
	d.Phonenumber = a.Phonenumber
	d.PhotoUrl = a.PhotoUrl
	d.Email = a.Email
	return
}

func (s *frontcacheServer) ListCompanies(ctx context.Context, req *company.CompanyListRequest) (*company.CompanyList, error) {
	defer helpers.Duration(helpers.Track("ListCompanies"))
	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := companyClient.ListCompanies(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error list company")
		}
		return res, nil
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	rows, err := companyClient.ListCompanyRows(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "err ListCompanyRows")
	}

	res := &company.CompanyList{Limit: req.Limit, Offset: req.Offset}
	for _, id := range rows.CompanyUuid {
		r := &company.GetCompanyRequest{Uuid: id}
		var c *company.Company
		if c, err = s.GetCompany(ctx, r); err != nil {
			return nil, err
		}
		res.Companies = append(res.Companies, *c)
	}

	return res, nil
}

func (s *frontcacheServer) GetCompany(ctx context.Context, req *company.GetCompanyRequest) (*company.Company, error) {
	defer helpers.Duration(helpers.Track("GetCompany"))

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.CompanyVersion
		if !s.use_callback {
			v, err = companyClient.GetCompanyVersion(ctx, &company.GetCompanyVersionRequest{Uuid: req.Uuid})
			if err != nil {
				return nil, s.internalError(err, "error getting company version")
			}
		}

		if res, ok := s.company_cache[req.Uuid]; ok {
			if !s.use_callback && int(v.CompanyVer) > int(res.Version) {
				s.logger.Info("Front Cache GetCompany version outdated cache miss")
			} else {
				s.logger.Info("Front Cache GetCompany cache hit [company uuid:" + req.Uuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache GetCompany cache miss [company uuid:" + req.Uuid + "]")
		}
	}

	c, err := companyClient.GetCompany(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting company")
	}

	if s.use_caching {
		s.company_lock.Lock()
		s.company_cache[req.Uuid] = c
		s.company_lock.Unlock()
	}

	return c, nil
}

func (s *frontcacheServer) ListTeams(ctx context.Context, req *company.TeamListRequest) (*company.TeamList, error) {
	defer helpers.Duration(helpers.Track("ListTeams"))

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.TeamsVersion
		if !s.use_callback {
			v, err = companyClient.GetTeamsVersion(ctx, &company.GetTeamsVersionRequest{Uuid: req.CompanyUuid})
			if err != nil {
				return nil, s.internalError(err, "error getting ListTeams version")
			}
		}

		if res, ok := s.teams_cache[req.CompanyUuid]; ok {
			if !s.use_callback && int(v.TeamsVer) > int(res.Version) {
				s.logger.Info("Front Cache ListTeams version outdated cache miss")
			} else {
				s.logger.Info("Front Cache ListTeams cache hit [company uuid:" + req.CompanyUuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache ListTeams cache miss [company uuid:" + req.CompanyUuid + "]")
		}
	}

	t, err := companyClient.ListTeams(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting team list")
	}

	if s.use_caching {
		s.teams_lock.Lock()
		s.teams_cache[req.CompanyUuid] = t
		s.teams_lock.Unlock()

		s.team_lock.Lock()
		for _, c := range t.Teams {
			s.team_cache[c.Uuid] = &c
		}
		s.team_lock.Unlock()
	}
	return t, nil
}

func (s *frontcacheServer) GetTeam(ctx context.Context, req *company.GetTeamRequest) (*company.Team, error) {
	defer helpers.Duration(helpers.Track("GetTeam"))
	if _, err := s.GetCompany(ctx, &company.GetCompanyRequest{Uuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.TeamVersion
		if !s.use_callback {
			v, err = companyClient.GetTeamVersion(ctx, &company.GetTeamVersionRequest{Uuid: req.Uuid})
			if err != nil {
				return nil, s.internalError(err, "error getting GetTeam version")
			}
		}

		if res, ok := s.team_cache[req.Uuid]; ok {
			if !s.use_callback && int(v.TeamVer) > int(res.Version) {
				s.logger.Info("Front Cache GetTeam version outdated cache miss")
			} else {
				s.logger.Info("Front Cache GetTeam cache hit [team uuid:" + req.Uuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache GetTeam cache miss [team uuid:" + req.Uuid + "]")
		}
	}

	t, err := companyClient.GetTeam(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting team")
	}

	if s.use_caching {
		s.team_lock.Lock()
		s.team_cache[req.Uuid] = t
		s.team_lock.Unlock()
	}
	return t, nil
}

func (s *frontcacheServer) GetWorkerTeamInfo(ctx context.Context, req *company.Worker) (*company.Worker, error) {
	defer helpers.Duration(helpers.Track("GetWorkerTeamInfo"))

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.WorkerTeamVersion
		if !s.use_callback {
			v, err = companyClient.GetWorkerTeamVersion(ctx, &company.GetWorkerTeamVersionRequest{Uuid: req.UserUuid})
			if err != nil {
				return nil, s.internalError(err, "error getting GetWorkerTeam version")
			}
		}

		if res, ok := s.workerteam_cache[req.UserUuid]; ok {
			if !s.use_callback && int(v.WorkerteamVer) > int(res.Version) {
				s.logger.Info("Front Cache GetWorkerTeamInfo version outdated cache miss")
			} else {
				s.logger.Info("Front Cache GetWorkerTeamInfo cache hit [user uuid:" + req.UserUuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache GetWorkerTeamInfo cache miss [user uuid:" + req.UserUuid + "]")
		}
	}

	w, err := companyClient.GetWorkerTeamInfo(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting workerteaminfo")
	}

	if s.use_caching {
		s.workerteam_lock.Lock()
		s.workerteam_cache[req.UserUuid] = w
		s.workerteam_lock.Unlock()
	}
	return w, nil
}

func (s *frontcacheServer) ListJobs(ctx context.Context, req *company.JobListRequest) (*company.JobList, error) {
	defer helpers.Duration(helpers.Track("ListJobs"))
	if _, err := s.GetTeam(ctx, &company.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.JobsVersion
		if !s.use_callback {
			v, err = companyClient.GetJobsVersion(ctx, &company.GetJobsVersionRequest{Uuid: req.TeamUuid})
			if err != nil {
				return nil, s.internalError(err, "error getting ListJobs version")
			}
		}

		if res, ok := s.jobs_cache[req.TeamUuid]; ok {
			if !s.use_callback && int(v.JobsVer) > int(res.Version) {
				s.logger.Info("Front Cache ListJobs version outdated cache miss")
			} else {
				s.logger.Info("Front Cache ListJobs cache hit [user uuid:" + req.TeamUuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache ListJobs cache miss [user uuid:" + req.TeamUuid + "]")
		}
	}

	j, err := companyClient.ListJobs(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting workerteaminfo")
	}

	if s.use_caching {
		s.jobs_lock.Lock()
		s.jobs_cache[req.TeamUuid] = j
		s.jobs_lock.Unlock()

		s.job_lock.Lock()
		for _, c := range j.Jobs {
			s.job_cache[c.Uuid] = &c
		}
		s.job_lock.Unlock()
	}
	return j, nil
}

func (s *frontcacheServer) GetJob(ctx context.Context, req *company.GetJobRequest) (*company.Job, error) {
	defer helpers.Duration(helpers.Track("GetJob"))
	if _, err := s.GetTeam(ctx, &company.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.JobVersion
		if !s.use_callback {
			v, err = companyClient.GetJobVersion(ctx, &company.GetJobVersionRequest{Uuid: req.Uuid})
			if err != nil {
				return nil, s.internalError(err, "error getting GetJob version")
			}
		}

		if res, ok := s.job_cache[req.Uuid]; ok {
			if !s.use_callback && int(v.JobVer) > int(res.Version) {
				s.logger.Info("Front Cache GetJob version outdated cache miss")
			} else {
				s.logger.Info("Front Cache GetJob cache hit [job uuid:" + req.Uuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache GetJob cache miss [job uuid:" + req.Uuid + "]")
		}
	}

	j, err := companyClient.GetJob(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting workerteaminfo")
	}

	if s.use_caching {
		s.job_lock.Lock()
		s.job_cache[req.Uuid] = j
		s.job_lock.Unlock()
	}
	return j, nil
}

func (s *frontcacheServer) Directory(ctx context.Context, req *company.DirectoryListRequest) (*company.DirectoryList, error) {
	defer helpers.Duration(helpers.Track("Directory"))
	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := companyClient.Directory(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error directory")
		}
		return res, nil
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	rows, err := companyClient.ListDirectoryRows(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "err scanning database")
	}

	res := &company.DirectoryList{Limit: req.Limit, Offset: req.Offset}
	for _, ids := range rows.DirectoryIds {
		e := &company.DirectoryEntry{CompanyUuid: req.CompanyUuid, InternalId: ids.InternalId, UserUuid: ids.UserUuid}
		a, err := s.GetAccount(ctx, &account.GetAccountRequest{Uuid: e.UserUuid})
		if err != nil {
			return nil, s.internalError(err, "error get account")
		}
		copyAccountToDirectory(a, e)
		res.Accounts = append(res.Accounts, *e)
	}

	return res, nil
}

func (s *frontcacheServer) GetAssociations(ctx context.Context, req *company.DirectoryListRequest) (*company.AssociationList, error) {
	defer helpers.Duration(helpers.Track("GetAssociations"))
	d, err := s.Directory(ctx, req)
	if err != nil {
		return nil, err
	}

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	res := &company.AssociationList{Offset: req.Offset, Limit: req.Limit}
	for _, e := range d.Accounts {
		a := &company.Association{Account: e}
		teams, err := companyClient.GetWorkerOf(ctx, &company.WorkerOfRequest{UserUuid: e.UserUuid})
		if err != nil {
			return nil, err
		}
		for _, team := range teams.Teams {
			if team.CompanyUuid == req.CompanyUuid {
				a.Teams = append(a.Teams, team)
			}
		}

		_, err = s.GetAdmin(ctx, &company.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: e.UserUuid})
		switch {
		case err == nil:
			a.Admin = true
		case grpc.Code(err) == codes.NotFound:
			a.Admin = false
		default:
			s.internalError(err, "failed to fetch admin")
		}

		res.Accounts = append(res.Accounts, *a)

	}
	return res, nil
}

func (s *frontcacheServer) GetDirectoryEntry(ctx context.Context, req *company.DirectoryEntryRequest) (*company.DirectoryEntry, error) {
	defer helpers.Duration(helpers.Track("GetDirectoryEntry"))
	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := companyClient.GetDirectoryEntry(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error getting directory entry")
		}
		return res, nil
	}

	id, err := companyClient.GetDirectoryEntryID(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "err scanning database")
	}

	res := &company.DirectoryEntry{UserUuid: req.UserUuid, CompanyUuid: req.CompanyUuid, InternalId: id.InternalId}
	a, err := s.GetAccount(ctx, &account.GetAccountRequest{Uuid: res.UserUuid})
	if err != nil {
		return nil, s.internalError(err, "error get account")
	}
	copyAccountToDirectory(a, res)
	return res, nil
}

func (s *frontcacheServer) ListAdmins(ctx context.Context, req *company.AdminListRequest) (*company.Admins, error) {
	defer helpers.Duration(helpers.Track("ListAdmins"))

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.AdminsVersion
		if !s.use_callback {
			v, err = companyClient.GetAdminsVersion(ctx, &company.GetAdminsVersionRequest{Uuid: req.CompanyUuid})
			if err != nil {
				return nil, s.internalError(err, "error getting ListAdmins version")
			}
		}

		if res, ok := s.admins_cache[req.CompanyUuid]; ok {
			if !s.use_callback && int(v.AdminsVer) > int(res.Version) {
				s.logger.Info("Front Cache ListAdmins version outdated cache miss")
			} else {
				s.logger.Info("Front Cache ListAdmins cache hit [user uuid:" + req.CompanyUuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache ListAdmins cache miss [user uuid:" + req.CompanyUuid + "]")
		}
	}

	admin, err := companyClient.ListAdmins(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting listadmins")
	}

	if s.use_caching {
		s.admins_lock.Lock()
		s.admins_cache[req.CompanyUuid] = admin
		s.admins_lock.Unlock()
	}
	return admin, nil
}

func (s *frontcacheServer) GetAdmin(ctx context.Context, req *company.DirectoryEntryRequest) (*company.DirectoryEntry, error) {
	defer helpers.Duration(helpers.Track("GetAdmin"))
	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := companyClient.GetAdmin(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error getting admin")
		}
		return res, nil
	}

	exist, err := companyClient.GetAdminExist(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "err scanning database")
	}
	if exist.Exist {
		return s.GetDirectoryEntry(ctx, req)
	} else {
		return nil, grpc.Errorf(codes.NotFound, "admin relationship not found")
	}
}

func (s *frontcacheServer) ListWorkers(ctx context.Context, req *company.WorkerListRequest) (*company.Workers, error) {
	defer helpers.Duration(helpers.Track("ListWorkers"))
	if _, err := s.GetTeam(ctx, &company.GetTeamRequest{CompanyUuid: req.CompanyUuid, Uuid: req.TeamUuid}); err != nil {
		return nil, err
	}

	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if s.use_caching {
		var v *company.WorkersVersion
		if !s.use_callback {
			v, err = companyClient.GetWorkersVersion(ctx, &company.GetWorkersVersionRequest{Uuid: req.TeamUuid})
			if err != nil {
				return nil, s.internalError(err, "error getting ListAdmins version")
			}
		}

		if res, ok := s.workers_cache[req.TeamUuid]; ok {
			if !s.use_callback && int(v.WorkersVer) > int(res.Version) {
				s.logger.Info("Front Cache ListWorkers version outdated cache miss")
			} else {
				s.logger.Info("Front Cache ListWorkers cache hit [user uuid:" + req.TeamUuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache ListWorkers cache miss [user uuid:" + req.TeamUuid + "]")
		}
	}

	w, err := companyClient.ListWorkers(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting list workers")
	}

	if s.use_caching {
		s.workers_lock.Lock()
		s.workers_cache[req.TeamUuid] = w
		s.workers_lock.Unlock()
	}
	return w, nil
}

func (s *frontcacheServer) GetWorker(ctx context.Context, req *company.Worker) (*company.DirectoryEntry, error) {
	defer helpers.Duration(helpers.Track("GetWorker"))
	companyClient, close, err := company.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := companyClient.GetWorker(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error getting worker")
		}
		return res, nil
	}

	exist, err := companyClient.GetWorkerexist(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting list workers")
	}
	if exist.Exist {
		return s.GetDirectoryEntry(ctx, &company.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: req.UserUuid})
	} else {
		return nil, grpc.Errorf(codes.NotFound, "worker relationship not found")
	}
}

func (s *frontcacheServer) ListAccounts(ctx context.Context, req *account.GetAccountListRequest) (*account.AccountList, error) {
	defer helpers.Duration(helpers.Track("ListAccounts"))
	accountClient, close, err := account.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init company connection")
	}
	defer close()

	if !s.use_caching {
		res, err := accountClient.List(ctx, req)
		if err != nil {
			return nil, s.internalError(err, "error list accounts")
		}
		return res, nil
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	rows, err := accountClient.ListAccountRows(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "err ListCompanyRows")
	}

	res := &account.AccountList{Limit: req.Limit, Offset: req.Offset}
	for _, id := range rows.Uuid {
		r := &account.GetAccountRequest{Uuid: id}
		var a *account.Account
		if a, err = s.GetAccount(ctx, r); err != nil {
			return nil, err
		}
		res.Accounts = append(res.Accounts, *a)
	}
	return res, nil
}

func (s *frontcacheServer) GetAccount(ctx context.Context, req *account.GetAccountRequest) (*account.Account, error) {
	defer helpers.Duration(helpers.Track("GetAccount"))

	accountClient, close, err := account.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to init account connection")
	}
	defer close()

	if s.use_caching {
		var v *account.AccountVersion
		if !s.use_callback {
			v, err = accountClient.GetAccountVersion(ctx, &account.GetAccountVersionRequest{Uuid: req.Uuid})
			if err != nil {
				return nil, s.internalError(err, "error getting GetAccount version")
			}
		}

		if res, ok := s.account_cache[req.Uuid]; ok {
			if !s.use_callback && int(v.AccountVer) > int(res.Version) {
				s.logger.Info("Front Cache GetAccount version outdated cache miss")
			} else {
				s.logger.Info("Front Cache GetAccount cache hit [account uuid:" + req.Uuid + "]")
				return res, nil
			}
		} else {
			s.logger.Info("Front Cache GetAccount cache miss [account uuid:" + req.Uuid + "]")
		}
	}

	a, err := accountClient.Get(ctx, req)
	if err != nil {
		return nil, s.internalError(err, "error getting list workers")
	}

	if s.use_caching {
		s.account_lock.Lock()
		s.account_cache[req.Uuid] = a
		s.account_lock.Unlock()
	}
	return a, nil
}

func (s *frontcacheServer) InvalidateWorkersCache(ctx context.Context, req *frontcache.InvalidateWorkersCacheRequest) (*empty.Empty, error) {
	if _, ok := s.workers_cache[req.TeamUuid]; ok {
		s.workers_lock.Lock()
		delete(s.workers_cache, req.TeamUuid)
		s.workers_lock.Unlock()
		s.logger.Info("FrontCache delete workers [team uuid:" + req.TeamUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateJobsCache(ctx context.Context, req *frontcache.InvalidateJobsCacheRequest) (*empty.Empty, error) {
	if _, ok := s.jobs_cache[req.TeamUuid]; ok {
		s.jobs_lock.Lock()
		delete(s.jobs_cache, req.TeamUuid)
		s.jobs_lock.Unlock()
		s.logger.Info("FrontCache delete jobs [team uuid:" + req.TeamUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateJobCache(ctx context.Context, req *frontcache.InvalidateJobCacheRequest) (*empty.Empty, error) {
	if _, ok := s.job_cache[req.JobUuid]; ok {
		s.job_lock.Lock()
		delete(s.job_cache, req.JobUuid)
		s.job_lock.Unlock()
		s.logger.Info("FrontCache delete job [team uuid:" + req.JobUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateCompanyCache(ctx context.Context, req *frontcache.InvalidateCompanyCacheRequest) (*empty.Empty, error) {
	if _, ok := s.company_cache[req.CompanyUuid]; ok {
		s.company_lock.Lock()
		delete(s.company_cache, req.CompanyUuid)
		s.company_lock.Unlock()
		s.logger.Info("FrontCache delete company [company uuid:" + req.CompanyUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateTeamsCache(ctx context.Context, req *frontcache.InvalidateTeamsCacheRequest) (*empty.Empty, error) {
	if _, ok := s.teams_cache[req.CompanyUuid]; ok {
		s.teams_lock.Lock()
		delete(s.teams_cache, req.CompanyUuid)
		s.teams_lock.Unlock()
		s.logger.Info("FrontCache delete teams [company uuid:" + req.CompanyUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateTeamCache(ctx context.Context, req *frontcache.InvalidateTeamCacheRequest) (*empty.Empty, error) {
	if _, ok := s.team_cache[req.TeamUuid]; ok {
		s.team_lock.Lock()
		delete(s.team_cache, req.TeamUuid)
		s.team_lock.Unlock()
		s.logger.Info("FrontCache delete team [team uuid:" + req.TeamUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateAdminsCache(ctx context.Context, req *frontcache.InvalidateAdminsCacheRequest) (*empty.Empty, error) {
	if _, ok := s.admins_cache[req.CompanyUuid]; ok {
		s.admins_lock.Lock()
		delete(s.admins_cache, req.CompanyUuid)
		s.admins_lock.Unlock()
		s.logger.Info("FrontCache delete admins [team uuid:" + req.CompanyUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateWorkerteamCache(ctx context.Context, req *frontcache.InvalidateWorkerteamCacheRequest) (*empty.Empty, error) {
	if _, ok := s.workerteam_cache[req.WorkerUuid]; ok {
		s.workerteam_lock.Lock()
		delete(s.workerteam_cache, req.WorkerUuid)
		s.workerteam_lock.Unlock()
		s.logger.Info("FrontCache delete workerteam [worker uuid:" + req.WorkerUuid + "]")
	}
	return &empty.Empty{}, nil
}

func (s *frontcacheServer) InvalidateAccountCache(ctx context.Context, req *frontcache.InvalidateAccountCacheRequest) (*empty.Empty, error) {
	if _, ok := s.account_cache[req.AccountUuid]; ok {
		s.account_lock.Lock()
		delete(s.account_cache, req.AccountUuid)
		s.account_lock.Unlock()
		s.logger.Info("FrontCache delete account [account uuid:" + req.AccountUuid + "]")
	}
	return &empty.Empty{}, nil
}
