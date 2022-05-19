package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/protobuf/ptypes/empty"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"v2.staffjoy.com/account"
	"v2.staffjoy.com/auth"
	"v2.staffjoy.com/bot"
	pb "v2.staffjoy.com/company"
	"v2.staffjoy.com/helpers"
)

func (s *companyServer) CreateDirectory(ctx context.Context, req *pb.NewDirectoryEntry) (*pb.DirectoryEntry, error) {
	md, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "failed to authorize")
	// }

	// switch authz {
	// case auth.AuthorizationSupportUser:
	// case auth.AuthorizationAuthenticatedUser:
	// 	if err = s.PermissionCompanyAdmin(md, req.CompanyUuid); err != nil {
	// 		return nil, err
	// 	}
	// case auth.AuthorizationWWWService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "you do not have access to this service")
	// }

	if _, err = s.GetCompany(ctx, &pb.GetCompanyRequest{Uuid: req.CompanyUuid}); err != nil {
		return nil, err
	}
	createMd := metadata.New(map[string]string{auth.AuthorizationMetadata: auth.AuthorizationWWWService})
	newCtx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), createMd))
	defer cancel()

	accountClient, close, err := account.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to initiate account connection")
	}
	defer close()

	a, err := accountClient.GetOrCreate(newCtx, &account.GetOrCreateRequest{Email: req.Email, Name: req.Name, Phonenumber: req.Phonenumber})
	if err != nil {
		return nil, s.internalError(err, "could not get or create user")
	}

	d := &pb.DirectoryEntry{InternalId: req.InternalId, CompanyUuid: req.CompanyUuid}
	copyAccountToDirectory(a, d)

	var exists bool
	err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM directory WHERE (company_uuid=? AND user_uuid=?))", req.CompanyUuid, a.Uuid).Scan(&exists)
	if err != nil {
		return nil, s.internalError(err, "failed to query database")
	} else if exists {
		return nil, grpc.Errorf(codes.AlreadyExists, "relationship already exists")
	}
	_, err = s.db.Exec("INSERT INTO directory (company_uuid, user_uuid, internal_id) values (?, ?, ?)",
		req.CompanyUuid, a.Uuid, req.InternalId)
	if err != nil {
		return nil, s.internalError(err, "could not create entry")
	}

	al := newAuditEntry(md, "directory", d.UserUuid, req.CompanyUuid, "")
	al.UpdatedContents = d
	al.Log(logger, "updated directory")

	go func() {
		botClient, close, err := bot.NewClient()
		if err != nil {
			s.internalError(err, "unable to initiate bot connection")
			return
		}
		defer close()
		if _, err := botClient.OnboardWorker(asyncContext(), &bot.OnboardWorkerRequest{CompanyUuid: req.CompanyUuid, UserUuid: d.UserUuid}); err != nil {
			s.internalError(err, "failed to onboard worker")
		}
	}()

	go helpers.TrackEventFromMetadata(md, "directoryentry_created")

	return d, nil
}

func (s *companyServer) Directory(ctx context.Context, req *pb.DirectoryListRequest) (*pb.DirectoryList, error) {
	defer helpers.Duration(helpers.Track("Directory"))
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
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	if req.Limit <= 0 {
		req.Limit = 20
	}

	res := &pb.DirectoryList{Limit: req.Limit, Offset: req.Offset}

	rows, err := s.db.Query("select internal_id, user_uuid from directory WHERE company_uuid=? limit ? offset ?", req.CompanyUuid, req.Limit, req.Offset)
	if err != nil {
		return nil, s.internalError(err, "unable to query database")
	}

	for rows.Next() {
		e := &pb.DirectoryEntry{CompanyUuid: req.CompanyUuid}
		err := rows.Scan(&e.InternalId, &e.UserUuid)
		if err != nil {
			return nil, s.internalError(err, "error scanning database")
		}

		if s.use_caching {
			if a, ok := s.account_cache[e.UserUuid]; ok {
				copyAccountToDirectory(a, e)
				res.Accounts = append(res.Accounts, *e)
				s.logger.Info("directory account cache hit [account uuid:" + e.UserUuid + "]")
				continue
			} else {
				s.logger.Info("directory account cache miss [account uuid:" + e.UserUuid + "]")
			}
		}

		md := metadata.New(map[string]string{auth.AuthorizationMetadata: auth.AuthorizationCompanyService})
		newCtx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), md))
		defer cancel()

		accountClient, close, err := account.NewClient()
		if err != nil {
			return nil, s.internalError(err, "unable to initiate account connection")
		}
		defer close()

		a, err := accountClient.Get(newCtx, &account.GetAccountRequest{Uuid: e.UserUuid})
		if err != nil {
			return nil, s.internalError(err, "error scanning database")
		}

		if s.use_caching {
			s.account_lock.Lock()
			s.account_cache[e.UserUuid] = a
			s.account_lock.Unlock()
		}

		copyAccountToDirectory(a, e)
		res.Accounts = append(res.Accounts, *e)
	}
	return res, nil
}

func (s *companyServer) GetDirectoryEntry(ctx context.Context, req *pb.DirectoryEntryRequest) (*pb.DirectoryEntry, error) {
	defer helpers.Duration(helpers.Track("GetDirectory"))
	_, _, err := getAuth(ctx)
	// if err != nil {
	// 	return nil, s.internalError(err, "Failed to authorize")
	// }

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
	// case auth.AuthorizationWhoamiService:
	// case auth.AuthorizationWWWService:
	// default:
	// 	return nil, grpc.Errorf(codes.PermissionDenied, "You do not have access to this service")
	// }

	e := &pb.DirectoryEntry{UserUuid: req.UserUuid, CompanyUuid: req.CompanyUuid}
	err = s.db.QueryRow("SELECT internal_id from directory WHERE (company_uuid=? AND user_uuid=?) LIMIT 1", req.CompanyUuid, req.UserUuid).Scan(&e.InternalId)
	if err == sql.ErrNoRows {
		return nil, grpc.Errorf(codes.NotFound, "directory entry not found for user in this company")
	} else if err != nil {
		return nil, s.internalError(err, "failed to query database")
	}

	if s.use_caching {
		if a, ok := s.account_cache[e.UserUuid]; ok {
			copyAccountToDirectory(a, e)
			s.logger.Info("get directory entry account cache hit [account uuid:" + e.UserUuid + "]")
			return e, nil
		} else {
			s.logger.Info("get directory entry account cache miss [account uuid:" + e.UserUuid + "]")
		}
	}

	newMD := metadata.New(map[string]string{auth.AuthorizationMetadata: auth.AuthorizationCompanyService})
	newCtx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), newMD))
	defer cancel()

	accountClient, close, err := account.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to initiate account connection")
	}
	defer close()

	a, err := accountClient.Get(newCtx, &account.GetAccountRequest{Uuid: e.UserUuid})
	if err != nil {
		return nil, s.internalError(err, "error fetching account")
	}

	if s.use_caching {
		s.account_lock.Lock()
		s.account_cache[e.UserUuid] = a
		s.account_lock.Unlock()
	}
	copyAccountToDirectory(a, e)
	return e, nil
}

func (s *companyServer) UpdateDirectoryEntry(ctx context.Context, req *pb.DirectoryEntry) (*pb.DirectoryEntry, error) {
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

	orig, err := s.GetDirectoryEntry(ctx, &pb.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: req.UserUuid})
	if err != nil {
		return nil, grpc.Errorf(codes.NotFound, "entry does not exist")
	}

	authMd := metadata.New(map[string]string{auth.AuthorizationMetadata: auth.AuthorizationCompanyService})
	newCtx, cancel := context.WithCancel(metadata.NewOutgoingContext(context.Background(), authMd))
	defer cancel()

	accountClient, close, err := account.NewClient()
	if err != nil {
		return nil, s.internalError(err, "unable to initiate account connection")
	}
	defer close()

	a, err := accountClient.Get(newCtx, &account.GetAccountRequest{Uuid: orig.UserUuid})
	if err != nil {
		return nil, s.internalError(err, "error fetching account")
	}

	var accountUpdateRequested bool
	switch {
	case req.Name != orig.Name:
		fallthrough
	case req.Email != orig.Email:
		fallthrough
	case req.Phonenumber != orig.Phonenumber:
		accountUpdateRequested = true
	}

	if a.ConfirmedAndActive && accountUpdateRequested {
		return nil, grpc.Errorf(codes.InvalidArgument, "this user is active, so they cannot be modified")
	} else if a.Support && accountUpdateRequested {
		return nil, grpc.Errorf(codes.PermissionDenied, "you cannot change this account")
	}

	if accountUpdateRequested {
		a.Name = req.Name
		a.Phonenumber = req.Phonenumber
		a.Email = req.Email
		if _, err := accountClient.Update(newCtx, a); err != nil {
			return nil, err
		}
		copyAccountToDirectory(a, req)
	}

	if _, err = s.db.Exec("UPDATE directory SET internal_id=? WHERE (user_uuid=? AND company_uuid=?)", req.InternalId, req.UserUuid, req.CompanyUuid); err != nil {
		return nil, s.internalError(err, "Failed to query database")
	}
	al := newAuditEntry(md, "directory", a.Uuid, req.CompanyUuid, "")
	al.OriginalContents = orig
	al.UpdatedContents = req
	al.Log(logger, "updated directory entry for account")

	go func() {
		if !req.ConfirmedAndActive && ((orig.Phonenumber != req.Phonenumber) || (req.Phonenumber == "" && orig.Email != req.Email)) {
			botClient, close, err := bot.NewClient()
			if err != nil {
				s.internalError(err, "unable to initiate bot connection")
				return
			}
			defer close()
			_, err = botClient.OnboardWorker(asyncContext(), &bot.OnboardWorkerRequest{CompanyUuid: req.CompanyUuid, UserUuid: req.UserUuid})
			if err != nil {
				s.internalError(err, "failed to onboard worker")
			}
		}
	}()
	go helpers.TrackEventFromMetadata(md, "directoryentry_updated")

	return req, nil
}

func (s *companyServer) GetAssociations(ctx context.Context, req *pb.DirectoryListRequest) (*pb.AssociationList, error) {
	// this handles permissions
	d, err := s.Directory(ctx, req)
	if err != nil {
		return nil, err
	}

	res := &pb.AssociationList{Offset: req.Offset, Limit: req.Limit}
	for _, e := range d.Accounts {
		a := &pb.Association{Account: e}
		teams, err := s.GetWorkerOf(ctx, &pb.WorkerOfRequest{UserUuid: e.UserUuid})
		if err != nil {
			return nil, err
		}
		for _, team := range teams.Teams {
			if team.CompanyUuid == req.CompanyUuid {
				a.Teams = append(a.Teams, team)
			}
		}

		_, err = s.GetAdmin(ctx, &pb.DirectoryEntryRequest{CompanyUuid: req.CompanyUuid, UserUuid: e.UserUuid})
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

func (s *companyServer) InvalidateCache(ctx context.Context, req *pb.InvalidateCacheRequest) (*empty.Empty, error) {
	if _, ok := s.account_cache[req.UserUuid]; ok {
		s.account_lock.Lock()
		delete(s.account_cache, req.UserUuid)
		s.account_lock.Unlock()
		s.logger.Info("Invalidate cache [account uuid:" + req.UserUuid + "]")
	}

	rows, err := s.db.Query("select company_uuid from admin where user_uuid=?", req.UserUuid)
	if err != nil {
		return nil, s.internalError(err, "Unable to query database")
	}
	for rows.Next() {
		var companyUUID string
		if err := rows.Scan(&companyUUID); err != nil {
			return nil, s.internalError(err, "err scanning database")
		}
		if _, ok := s.admins_cache[companyUUID]; ok {
			s.admins_lock.Lock()
			delete(s.admins_cache, companyUUID)
			s.admins_lock.Unlock()
		}
	}
	return &empty.Empty{}, nil
}
