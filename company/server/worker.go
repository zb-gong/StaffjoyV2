package main

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/golang/protobuf/ptypes/empty"
	pb "v2.staffjoy.com/company"
	"v2.staffjoy.com/helpers"
)

func (s *companyServer) ListWorkers(ctx context.Context, req *pb.WorkerListRequest) (*pb.Workers, error) {
	if s.use_caching {
		res, ok := s.workers_cache[req.TeamUuid]
		if ok {
			s.logger.Info("list worker cache hit")
			return res, nil
		} else {
			s.logger.Info("list worker cache miss")
		}
	}

	// Prep
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
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{CompanyUuid: req.CompanyUuid, Uuid: req.TeamUuid}); err != nil {
		return nil, err
	}

	res := &pb.Workers{CompanyUuid: req.CompanyUuid, TeamUuid: req.TeamUuid}

	rows, err := s.db.Query("select user_uuid from worker where team_uuid=?", req.TeamUuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	}

	for rows.Next() {
		var userUUID string
		if err := rows.Scan(&userUUID); err != nil {
			return nil, s.internalError(err, "Error scanning database")
		}
		e, err := s.GetDirectoryEntry(ctx, &pb.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: userUUID})
		if err != nil {
			return nil, err
		}
		res.Workers = append(res.Workers, *e)
	}

	if s.use_caching {
		s.workers_lock.Lock()
		s.workers_cache[req.TeamUuid] = res
		s.workers_lock.Unlock()
	}
	return res, nil
}

func (s *companyServer) GetWorker(ctx context.Context, req *pb.Worker) (*pb.DirectoryEntry, error) {
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
	// case auth.AuthorizationWWWService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "you do not have access to this service")
	// }
	if _, err = s.GetTeam(ctx, &pb.GetTeamRequest{CompanyUuid: req.CompanyUuid, Uuid: req.TeamUuid}); err != nil {
		return nil, err
	}

	var exists bool
	err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM worker WHERE (team_uuid=? AND user_uuid=?))", req.TeamUuid, req.UserUuid).Scan(&exists)
	if err != nil {
		return nil, s.internalError(err, "failed to query database")
	} else if !exists {
		return nil, grpc.Errorf(codes.NotFound, "worker relationship not found")
	}
	return s.GetDirectoryEntry(ctx, &pb.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: req.UserUuid})
}

func (s *companyServer) DeleteWorker(ctx context.Context, req *pb.Worker) (*empty.Empty, error) {
	md, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }

	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationSupportUser:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetWorker(ctx, req); err != nil {
		return nil, err
	}
	if _, err = s.db.Exec("DELETE from worker where (team_uuid=? AND user_uuid=?) LIMIT 1", req.TeamUuid, req.UserUuid); err != nil {
		return nil, s.internalError(err, "failed to query database")
	}
	al := newAuditEntry(md, "worker", req.UserUuid, req.CompanyUuid, req.TeamUuid)
	al.Log(logger, "removed worker")
	go helpers.TrackEventFromMetadata(md, "worker_deleted")

	if s.use_caching {
		s.workers_lock.Lock()
		if _, ok := s.workers_cache[req.TeamUuid]; ok {
			delete(s.workers_cache, req.TeamUuid)
		}
		s.workers_lock.Unlock()
	}
	return &empty.Empty{}, nil

}

func (s *companyServer) GetWorkerOf(ctx context.Context, req *pb.WorkerOfRequest) (*pb.WorkerOfList, error) {
	_, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }

	// switch authz {
	// case auth.AuthorizationAccountService:
	// case auth.AuthorizationWWWService:
	// case auth.AuthorizationAuthenticatedUser:
	// case auth.AuthorizationSupportUser:
	// 	//  This is an internal endpoint
	// case auth.AuthorizationWhoamiService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	res := &pb.WorkerOfList{UserUuid: req.UserUuid}

	rows, err := s.db.Query("select worker.team_uuid, team.company_uuid from worker JOIN team ON team.uuid=worker.team_uuid where worker.user_uuid=?", req.UserUuid)
	if err != nil {
		return nil, s.internalError(err, "Unable to query database")
	}

	for rows.Next() {
		var teamUUID, companyUUID string
		if err := rows.Scan(&teamUUID, &companyUUID); err != nil {
			return nil, s.internalError(err, "err scanning database")
		}
		t, err := s.GetTeam(ctx, &pb.GetTeamRequest{Uuid: teamUUID, CompanyUuid: companyUUID})
		if err != nil {
			return nil, err
		}
		res.Teams = append(res.Teams, *t)
	}

	return res, nil
}

func (s *companyServer) CreateWorker(ctx context.Context, req *pb.Worker) (*pb.DirectoryEntry, error) {
	md, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "failed to authorize")
	// }

	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationWhoamiService:
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationWWWService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err := s.GetTeam(ctx, &pb.GetTeamRequest{CompanyUuid: req.CompanyUuid, Uuid: req.TeamUuid}); err != nil {
		return nil, err
	}
	e, err := s.GetDirectoryEntry(ctx, &pb.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: req.UserUuid})
	if err != nil {
		return nil, err
	}

	_, err = s.GetWorker(ctx, req)
	if err == nil {
		return nil, grpc.Errorf(codes.AlreadyExists, "user is already a worker")
	} else if grpc.Code(err) != codes.NotFound {
		return nil, s.internalError(err, "an unknown error occurred while checking for existing worker relationships")
	}

	_, err = s.db.Exec("INSERT INTO worker (team_uuid, user_uuid) values (?, ?)", req.TeamUuid, req.UserUuid)
	if err != nil {
		return nil, s.internalError(err, "failed to query database")
	}
	al := newAuditEntry(md, "worker", req.UserUuid, req.CompanyUuid, req.TeamUuid)
	al.Log(logger, "added worker")
	go helpers.TrackEventFromMetadata(md, "worker_created")

	return e, nil
}
