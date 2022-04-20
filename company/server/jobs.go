package main

import (
	_ "github.com/go-sql-driver/mysql"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	pb "v2.staffjoy.com/company"
	"v2.staffjoy.com/crypto"
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
	j := &pb.Job{Uuid: uuid.String(), Name: req.Name, Color: req.Color, CompanyUuid: req.CompanyUuid, TeamUuid: req.TeamUuid}

	if err = s.dbMap.Insert(j); err != nil {
		return nil, s.internalError(err, "could not create job")
	}

	al := newAuditEntry(md, "job", j.Uuid, j.CompanyUuid, j.TeamUuid)
	al.UpdatedContents = j
	al.Log(logger, "created job")
	go helpers.TrackEventFromMetadata(md, "job_created")

	if s.use_caching {
		if _, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.jobs_lock.Lock()
			delete(s.jobs_cache, req.TeamUuid)
			s.jobs_lock.Unlock()
			s.logger.Info("create job [team uuid:%v] cache is invalidated", req.TeamUuid)
		}
	}

	return j, nil
}

func (s *companyServer) ListJobs(ctx context.Context, req *pb.JobListRequest) (*pb.JobList, error) {
	if s.use_caching {
		if res, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.logger.Info("list job cache hit")
			return res, nil
		} else {
			s.logger.Info("list job cache miss")
		}
	}

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

	res := &pb.JobList{}
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

	obj, err := s.dbMap.Get(pb.Job{}, req.Uuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	} else if obj == nil {
		return nil, grpc.Errorf(codes.NotFound, "job not found")
	}
	j := obj.(*pb.Job)
	j.CompanyUuid = req.CompanyUuid
	j.TeamUuid = req.TeamUuid
	return j, nil
}

func (s *companyServer) UpdateJob(ctx context.Context, req *pb.Job) (*pb.Job, error) {
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
		if _, ok := s.jobs_cache[orig.TeamUuid]; ok {
			s.jobs_lock.Lock()
			delete(s.jobs_cache, orig.TeamUuid)
			s.logger.Info("update job[orig %v] cache is invalidated", orig.TeamUuid)
			s.jobs_lock.Unlock()
		}
		if _, ok := s.jobs_cache[req.TeamUuid]; ok {
			s.jobs_lock.Lock()
			delete(s.jobs_cache, req.TeamUuid)
			s.logger.Info("update job[req %v] cache is invalidated", req.TeamUuid)
			s.jobs_lock.Unlock()
		}
	}
	return req, nil
}
