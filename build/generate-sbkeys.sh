#! /bin/bash
set -ex

mkdir certs
pushd certs
openssl req -newkey rsa:4096 -nodes -keyout pk.key -new -x509 -sha256 -days 3650 -subj "/CN=Platform Key/" -out pk.crt
openssl x509 -outform DER -in pk.crt -out pk.cer
openssl req -newkey rsa:4096 -nodes -keyout kek.key -new -x509 -sha256 -days 3650 -subj "/CN=Key Exchange Key/" -out kek.crt
openssl x509 -outform DER -in kek.crt -out kek.cer
openssl req -newkey rsa:4096 -nodes -keyout db.key -new -x509 -sha256 -days 3650 -subj "/CN=Signature Database key/" -out db.crt
openssl x509 -outform DER -in db.crt -out db.cer
popd