#!/bin/sh
set -eu

: "${W2PP_STORAGE_MANAGER_URL:=https://storage-manager-svc.herokuapp.com}"
: "${W2PP_STORAGE_MANAGER_BUCKET:=jeanluca-teste}"
export W2PP_STORAGE_MANAGER_URL W2PP_STORAGE_MANAGER_BUCKET

exec go run ./webserver/cmd/itemiconupload "$@"
