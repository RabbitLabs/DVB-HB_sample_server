@ECHO OFF
SET GOOS=linux
SET GOARCH=amd64
go build -o server_amd64
SET GOOS=windows
go build

