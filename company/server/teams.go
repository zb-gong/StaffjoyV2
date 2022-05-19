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

func (s *companyServer) CreateTeam(ctx context.Context, req *pb.CreateTeamRequest) (*pb.Team, error) {
	md, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }
	// switch authz {
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationWWWService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	c, err := s.GetCompany(ctx, &pb.GetCompanyRequest{Uuid: req.CompanyUuid})
	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "Company with specified id not found")
	}

	// sanitize
	if req.DayWeekStarts == "" {
		req.DayWeekStarts = c.DefaultDayWeekStarts
	} else if req.DayWeekStarts, err = sanitizeDayOfWeek(req.DayWeekStarts); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid DefaultDayWeekStarts")
	}
	if req.Timezone == "" {
		req.Timezone = c.DefaultTimezone
	} else if err = validTimezone(req.Timezone); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid timezone")
	}

	if err = validColor(req.Color); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid color")
	}

	uuid, err := crypto.NewUUID()
	if err != nil {
		return nil, s.internalError(err, "cannot generate a uuid")
	}
	t := &pb.Team{Uuid: uuid.String(), CompanyUuid: req.CompanyUuid, Name: req.Name, DayWeekStarts: req.DayWeekStarts, Timezone: req.Timezone, Color: req.Color}

	if err = s.dbMap.Insert(t); err != nil {
		return nil, s.internalError(err, "could not create team")
	}

	al := newAuditEntry(md, "team", t.Uuid, req.CompanyUuid, t.Uuid)
	al.UpdatedContents = t
	al.Log(logger, "created team")
	go helpers.TrackEventFromMetadata(md, "team_created")

	if s.use_caching {
		if _, ok := s.teams_cache[req.CompanyUuid]; ok {
			s.teams_lock.Lock()
			delete(s.teams_cache, req.CompanyUuid)
			s.teams_lock.Unlock()
			s.logger.Info("create team cache is invalidated [company uuid:" + req.CompanyUuid + "]")
		}
	}

	return t, nil
}

func (s *companyServer) ListTeams(ctx context.Context, req *pb.TeamListRequest) (*pb.TeamList, error) {
	defer helpers.Duration(helpers.Track("ListTeams"))
	if s.use_caching {
		if res, ok := s.teams_cache[req.CompanyUuid]; ok {
			s.logger.Info("list teams cache hit [company uuid:" + req.CompanyUuid + "]")
			return res, nil
		} else {
			s.logger.Info("list teams cache miss [company uuid:" + req.CompanyUuid + "]")
		}
	}

	_, _, err := getAuth(ctx)
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
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "you do not have access to this service")
	// }
	if _, err = s.GetCompany(ctx, &pb.GetCompanyRequest{Uuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	res := &pb.TeamList{}
	rows, err := s.db.Query("select uuid from team where company_uuid=?", req.CompanyUuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	}

	for rows.Next() {
		r := &pb.GetTeamRequest{CompanyUuid: req.CompanyUuid}
		if err := rows.Scan(&r.Uuid); err != nil {
			return nil, s.internalError(err, "error scanning database")
		}

		var t *pb.Team
		if t, err = s.GetTeam(ctx, r); err != nil {
			return nil, err
		}
		res.Teams = append(res.Teams, *t)
	}

	if s.use_caching {
		s.teams_lock.Lock()
		s.teams_cache[req.CompanyUuid] = res
		s.teams_lock.Unlock()
	}
	return res, nil
}

func (s *companyServer) GetTeam(ctx context.Context, req *pb.GetTeamRequest) (*pb.Team, error) {
	_, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }

	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionTeamWorker(md, req.CompanyUuid, req.Uuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationAccountService:
	// case auth.AuthorizationWhoamiService:
	// case auth.AuthorizationBotService:
	// case auth.AuthorizationWWWService:
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationICalService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if _, err = s.GetCompany(ctx, &pb.GetCompanyRequest{Uuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	obj, err := s.dbMap.Get(pb.Team{}, req.Uuid)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	} else if obj == nil {
		return nil, grpc.Errorf(codes.NotFound, "team not found")
	}
	t := obj.(*pb.Team)
	t.CompanyUuid = req.CompanyUuid
	return t, nil

}

func (s *companyServer) UpdateTeam(ctx context.Context, req *pb.Team) (*pb.Team, error) {
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

	// path
	if _, err = s.GetCompany(ctx, &pb.GetCompanyRequest{Uuid: req.CompanyUuid}); err != nil {
		return nil, err
	}

	// sanitize
	if req.DayWeekStarts, err = sanitizeDayOfWeek(req.DayWeekStarts); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid DefaultDayWeekStarts")
	}
	if err = validTimezone(req.Timezone); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid timezone")
	}
	if err = validColor(req.Color); err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid color")
	}

	t, err := s.GetTeam(ctx, &pb.GetTeamRequest{CompanyUuid: req.CompanyUuid, Uuid: req.Uuid})
	if err != nil {
		return nil, err
	}
	if _, err := s.dbMap.Update(req); err != nil {
		return nil, s.internalError(err, "could not update the team ")
	}

	al := newAuditEntry(md, "team", t.Uuid, req.CompanyUuid, t.Uuid)
	al.OriginalContents = t
	al.UpdatedContents = req
	al.Log(logger, "updated team")
	go helpers.TrackEventFromMetadata(md, "team_updated")

	if s.use_caching {
		if _, ok := s.teams_cache[t.CompanyUuid]; ok {
			s.teams_lock.Lock()
			delete(s.teams_cache, t.CompanyUuid)
			s.teams_lock.Unlock()
			s.logger.Info("update team cache is invalidated [orig:" + t.CompanyUuid + "]")
		}
		if _, ok := s.teams_cache[req.CompanyUuid]; ok {
			s.teams_lock.Lock()
			delete(s.teams_cache, req.CompanyUuid)
			s.teams_lock.Unlock()
			s.logger.Info("update team cache is invalidated [req:" + req.CompanyUuid + "]")
		}
	}

	return req, nil
}

// GetWorkerInfo is an internal API method that given a worker UUID will
// return team and company UUID - it's expected in the future that a
// worker might belong to multiple teams/companies so this will prob.
// need to be refactored at some point
func (s *companyServer) GetWorkerTeamInfo(ctx context.Context, req *pb.Worker) (*pb.Worker, error) {
	defer helpers.Duration(helpers.Track("GetWorkerTeamInfo"))
	if s.use_caching {
		if res, ok := s.workerteam_cache[req.UserUuid]; ok {
			s.logger.Info("workerteam get cache hit [user uuid:" + req.UserUuid + "]")
			return res, nil
		} else {
			s.logger.Info("workerteam get cache miss [user uuid:" + req.UserUuid + "]")
		}
	}
	_, _, err := getAuth(ctx)

	// switch authz {
	// case auth.AuthorizationAuthenticatedUser:
	// 	userUUID, err := auth.GetCurrentUserUUIDFromMetadata(md)
	// 	if err != nil {
	// 		return nil, s.internalError(err, "failed to find current user uuid")
	// 	}
	// 	// user can access their own entry
	// 	if userUUID != req.UserUuid {
	// 		if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationICalService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	teamUUID := ""
	q := "select team_uuid from worker where user_uuid = ?;"
	err = s.db.QueryRow(q, req.UserUuid).Scan(&teamUUID)
	if err != nil {
		logger.Debugf("get team -- %v", err)
		return nil, grpc.Errorf(codes.InvalidArgument, "Invalid user")
	}

	companyUUID := ""
	q = "select company_uuid from team where uuid = ?;"
	err = s.db.QueryRow(q, teamUUID).Scan(&companyUUID)
	if err != nil {
		logger.Debugf("get team -- %v", err)
		return nil, grpc.Errorf(codes.InvalidArgument, "invalid company")
	}

	w := &pb.Worker{
		CompanyUuid: companyUUID,
		TeamUuid:    teamUUID,
		UserUuid:    req.UserUuid,
	}

	if s.use_caching {
		s.workerteam_lock.Lock()
		s.workerteam_cache[req.UserUuid] = w
		s.workerteam_lock.Unlock()
	}
	return w, nil
}
