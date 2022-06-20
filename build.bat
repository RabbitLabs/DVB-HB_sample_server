@ECHO OFF
SET GOOS=linux
SET GOARCH=amd64
go build -o server_amd64
docker build . -t rabbitlabs/thumperproxy
docker login
docker push rabbitlabs/thumperproxy