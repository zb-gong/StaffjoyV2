package main

import (
	_ "github.com/go-sql-driver/mysql"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	pb "v2.staffjoy.com/company"
	"v2.staffjoy.com/crypto"
	"v2.staffjoy.com/frontcache"
	"v2.staffjoy.com/helpers"
)

func (s *companyServer) CreateJob(ctx context.Context, req *pb.CreateJobRequest) (*pb.Job, error) {
	md, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }
	// switch authz {
	// case auth.AuthorizationSupportUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationAuthenticatedUser:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	if err = validColor(req.Color); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid color")
	}

	uuid, err := crypto.NewUUID()
	if err != nil {
		return nil, s.internalError(err, "Cannot generate a uuid")
	}
	j := &pb.Job{Uuid: uuid.String(), Name: req.Name, Color: req.Color, CompanyUuid: req.CompanyUuid, TeamUuid: req.TeamUuid, Version: 0}

	if err = s.dbMap.Insert(j); err != nil {
		return nil, s.internalError(err, "could not create job")
	}

	al := newAuditEntry(md, "job", j.Uuid, j.CompanyUuid, j.TeamUuid)
	al.UpdatedContents = j
	al.Log(logger, "created job")
	go helpers.TrackEventFromMetadata(md, "job_created")

	if s.use_caching {
		s.job_lock.Lock()
		s.job_cache[j.Uuid] = j
		s.job_lock.Unlock()

		if js, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.jobs_lock.Lock()
			s.jobs_cache[req.TeamUuid].Jobs = append(js.Jobs, *j)
			if !s.use_callback {
				s.jobs_cache[req.TeamUuid].Version = js.Version + 1
			}
			s.jobs_lock.Unlock()
			s.logger.Info("CreateJob updates jobs list [team uuid:" + req.TeamUuid + "]")

			if s.use_callback {
				frontcacheClient, close, err := frontcache.NewClient()
				if err != nil {
					return nil, s.internalError(err, "unable to init frontcache connection")
				}
				defer close()

				_, err = frontcacheClient.InvalidateJobsCache(ctx, &frontcache.InvalidateJobsCacheRequest{TeamUuid: req.TeamUuid})
				if err != nil {
					return nil, s.internalError(err, "error invalidate FrontCache job list cache")
				}
			}
		}
	}

	return j, nil
}

func (s *companyServer) ListJobs(ctx context.Context, req *pb.JobListRequest) (*pb.JobList, error) {
	defer helpers.Duration(helpers.Track("ListJobs"))
	_, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "failed to authorize")
	// }
	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionTeamWorker(md, req.CompanyUuid, req.TeamUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationSupportUser:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "you do not have access to this service")
	// }
	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	if s.use_caching {
		if res, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.logger.Info("list job cache hit [team uuid:" + req.TeamUuid + "]")
			return res, nil
		} else {
			s.logger.Info("list job cache miss [team uuid:" + req.TeamUuid + "]")
		}
	}

	res := &pb.JobList{Version: 0}
	rows, err := s.db.Query("select uuid from job where team_uuid=?", req.TeamUuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	}

	for rows.Next() {
		r := &pb.GetJobRequest{CompanyUuid: req.CompanyUuid, TeamUuid: req.TeamUuid}
		if err := rows.Scan(&r.Uuid); err != nil {
			return nil, s.internalError(err, "error scanning database")
		}

		var j *pb.Job
		if j, err = s.GetJob(ctx, r); err != nil {
			return nil, err
		}
		res.Jobs = append(res.Jobs, *j)
	}

	if s.use_caching {
		s.jobs_lock.Lock()
		s.jobs_cache[req.TeamUuid] = res
		s.jobs_lock.Unlock()
	}
	return res, nil
}

func (s *companyServer) GetJob(ctx context.Context, req *pb.GetJobRequest) (*pb.Job, error) {
	_, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }
	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionTeamWorker(md, req.CompanyUuid, req.TeamUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationBotService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	if s.use_caching {
		if res, ok := s.job_cache[req.Uuid]; ok {
			s.logger.Info("GetJob cache hit [job uuid:" + req.Uuid + "]")
			return res, nil
		} else {
			s.logger.Info("GetJob cache miss [job uuid:" + req.Uuid + "]")
		}
	}

	obj, err := s.dbMap.Get(pb.Job{}, req.Uuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	} else if obj == nil {
		return nil, grpc.Errorf(codes.NotFound, "job not found")
	}
	j := obj.(*pb.Job)
	j.CompanyUuid = req.CompanyUuid
	j.TeamUuid = req.TeamUuid

	if s.use_caching {
		s.job_lock.Lock()
		s.job_cache[req.Uuid] = j
		s.job_lock.Unlock()
	}
	return j, nil
}

func (s *companyServer) UpdateJob(ctx context.Context, req *pb.Job) (*pb.Job, error) {
	defer helpers.Duration(helpers.Track("UpdateJob"))
	md, _, err := getAuth(ctx)
	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationSupportUser:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{Uuid: req.TeamUuid, CompanyUuid: req.CompanyUuid}); err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Company and team path not found")
	}

	if err = validColor(req.Color); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid color")
	}

	orig, err := s.GetJob(ctx, &pb.GetJobRequest{CompanyUuid: req.CompanyUuid, TeamUuid: req.TeamUuid, Uuid: req.Uuid})
	if err != nil {
		return nil, err
	}

	if _, err := s.dbMap.Update(req); err != nil {
		return nil, s.internalError(err, "could not update the job")
	}

	al := newAuditEntry(md, "job", req.Uuid, req.CompanyUuid, req.TeamUuid)
	al.OriginalContents = orig
	al.UpdatedContents = req
	al.Log(logger, "updated job")
	go helpers.TrackEventFromMetadata(md, "job_updated")

	// callback to the job list cache
	if s.use_caching {
		frontcacheClient, close, err := frontcache.NewClient()
		if err != nil {
			return nil, s.internalError(err, "unable to init frontcache connection")
		}
		defer close()

		// Can be optimized by judging whether original uuid equals to request uuid
		if js, ok := s.jobs_cache[orig.TeamUuid]; ok {
			s.jobs_lock.Lock()
			var index int
			for i, v := range js.Jobs {
				if v.Uuid == req.Uuid {
					index = i
					break
				}
			}
			s.jobs_cache[orig.TeamUuid].Jobs[index] = js.Jobs[len(js.Jobs)-1]
			s.jobs_cache[orig.TeamUuid].Jobs = s.jobs_cache[orig.TeamUuid].Jobs[:len(js.Jobs)-1]
			if !s.use_callback {
				s.jobs_cache[orig.TeamUuid].Version = js.Version + 1
			}
			s.jobs_lock.Unlock()
			s.logger.Info("UpdateJob udpates orig jobs cache [orig:" + orig.TeamUuid + "]")

			if s.use_callback {
				_, err = frontcacheClient.InvalidateJobsCache(ctx, &frontcache.InvalidateJobsCacheRequest{TeamUuid: orig.TeamUuid})
				if err != nil {
					return nil, s.internalError(err, "error invalidate FrontCache job list cache")
				}
			}
		}
		if js, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.jobs_lock.Lock()
			s.jobs_cache[req.TeamUuid].Jobs = append(js.Jobs, *req)
			if !s.use_callback && req.TeamUuid != orig.TeamUuid {
				s.jobs_cache[req.TeamUuid].Version = js.Version + 1
			}
			s.jobs_lock.Unlock()
			s.logger.Info("UpdateJob updates req jobs cache [req:" + req.TeamUuid + "]")

			if s.use_callback {
				_, err = frontcacheClient.InvalidateJobsCache(ctx, &frontcache.InvalidateJobsCacheRequest{TeamUuid: req.TeamUuid})
				if err != nil {
					return nil, s.internalError(err, "error invalidate FrontCache job list cache")
				}
			}
		}
		if js, ok := s.job_cache[orig.Uuid]; ok {
			s.job_lock.Lock()
			s.job_cache[orig.Uuid] = req
			if !s.use_callback {
				s.job_cache[orig.Uuid].Version = js.Version + 1
			}
			s.job_lock.Unlock()
			s.logger.Info("UpdateJob udpates job cache [job uuid:" + orig.Uuid + "]")

			if s.use_callback {
				_, err = frontcacheClient.InvalidateJobCache(ctx, &frontcache.InvalidateJobCacheRequest{JobUuid: orig.Uuid})
				if err != nil {
					return nil, s.internalError(err, "error invalidate FrontCache job cache")
				}
			}
		}
	}
	return req, nil
}

func (s *companyServer) GetJobsVersion(ctx context.Context, req *pb.GetJobsVersionRequest) (*pb.JobsVersion, error) {
	if res, ok := s.jobs_cache[req.Uuid]; ok {
		return &pb.JobsVersion{JobsVer: res.Version}, nil
	}
	return &pb.JobsVersion{JobsVer: 0}, nil
	// return nil, fmt.Errorf("GetJobsVersion not found req uuid")
}

func (s *companyServer) GetJobVersion(ctx context.Context, req *pb.GetJobVersionRequest) (*pb.JobVersion, error) {
	if res, ok := s.job_cache[req.Uuid]; ok {
		return &pb.JobVersion{JobVer: res.Version}, nil
	}
	return &pb.JobVersion{JobVer: 0}, nil
	// return nil, fmt.Errorf("GetJobVersion not found req uuid")
}
