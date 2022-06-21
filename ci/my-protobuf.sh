export GRPC_PATH=$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.15.2

# account
protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --go_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/account.proto
mv account/account.pb.go account/api/

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --grpc-gateway_out=logtostderr=true:../ \
    ./protobuf/account.proto
mv ./account/account.pb.gw.go ./account/api/

sed -i "s/package account/package main/g" account/api/account.pb.go
sed -i "s/package account/package main/g" account/api/account.pb.gw.go

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --gogo_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/account.proto

# company
protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --go_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/company.proto
mv company/company.pb.go company/api/

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --grpc-gateway_out=logtostderr=true:../ \
    ./protobuf/company.proto
mv ./company/company.pb.gw.go ./company/api/

sed -i "s/package company/package main/g" company/api/company.pb.go
sed -i "s/package company/package main/g" company/api/company.pb.gw.go

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --gogo_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/company.proto

# front end
protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --go_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/frontcache.proto
mv frontcache/frontcache.pb.go frontcache/api/

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --grpc-gateway_out=logtostderr=true:../ \
    ./protobuf/frontcache.proto
mv ./frontcache/frontcache.pb.gw.go ./frontcache/api/

sed -i "s/package frontcache/package main/g" frontcache/api/frontcache.pb.go
sed -i "s/package frontcache/package main/g" frontcache/api/frontcache.pb.gw.go

protoc \
    -I ./protobuf/ \
    -I ${GOPATH}/pkg/mod \
    -I ${GRPC_PATH}/third_party/googleapis \
    --gogo_out=Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations,plugins=grpc:../ \
    ./protobuf/frontcache.proto
