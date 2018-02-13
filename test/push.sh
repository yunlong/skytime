#!/bin/sh
boom -n 100 -c 10 -m POST -h "X-CSRF-Token: a-32-byte-long-key-goes-hererrrr" -d "{\"u\":[1,2,3,4,5],\"m\":{\"test\":1}}" http://127.0.0.1:8000/api/broadcast_all
#boom -n 10 -c 10 -m POST -h "X-Auth-Key:12345;X-Auth-Secret:secret"  -d "{\"u\":[1,2,3,4,5],\"m\":{\"test\":1}}" http://127.0.0.1:8000/push
